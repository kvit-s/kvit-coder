package mcp

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// stdioClient speaks JSON-RPC 2.0 to a subprocess over its stdin/stdout, the
// MCP "stdio" transport. Messages are newline-delimited JSON with no embedded
// newlines. A background goroutine reads stdout and feeds each line to the
// rpcEndpoint, which correlates responses by id.
type stdioClient struct {
	proto
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	endpoint *rpcEndpoint
	writeMu  sync.Mutex

	closeOnce sync.Once
	doneCh    chan struct{} // closed when the reader goroutine exits
}

// stdioBufSize bounds a single JSON-RPC line. MCP messages (tool schemas, tool
// results) can be large, so allow up to 16 MB per line.
const stdioBufSize = 16 * 1024 * 1024

// newStdioClient spawns the server subprocess and wires up the transport. It
// does not perform the handshake; the caller invokes Initialize under the
// startup deadline. cwd, when non-empty, is the subprocess working directory
// (servers that index their own working directory rely on this); empty inherits
// kvit-coder's working directory.
func newStdioClient(command string, args, extraEnv []string, cwd string, logger Logger) (*stdioClient, error) {
	if logger == nil {
		logger = nopLogger{}
	}
	if command == "" {
		return nil, fmt.Errorf("stdio transport requires a command")
	}

	cmd := exec.Command(command, expandArgs(args)...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	if len(extraEnv) > 0 {
		cmd.Env = append(cmd.Env, expandEnvEntries(extraEnv)...)
	}
	// Own process group so Close can reap the whole tree (npx → node → ...).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	// Surface server diagnostics (stderr) to debug logging rather than the
	// child inheriting ours, which would corrupt the agent's own output.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	c := &stdioClient{
		cmd:    cmd,
		stdin:  stdin,
		doneCh: make(chan struct{}),
	}
	c.endpoint = newRPCEndpoint(c.write, logger)
	c.proto = proto{conn: c.endpoint, logger: logger}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %q: %w", command, err)
	}

	go c.readLoop(stdout)
	go drainStderr(stderr, logger)

	return c, nil
}

// write serializes one JSON-RPC message followed by a newline to the
// subprocess stdin. Serialized to keep concurrent writes from interleaving.
func (c *stdioClient) write(payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := c.stdin.Write(payload); err != nil {
		return err
	}
	_, err := c.stdin.Write([]byte("\n"))
	return err
}

// readLoop reads newline-delimited messages from stdout and dispatches each.
// On EOF or read error it fails all pending calls so callers don't hang.
func (c *stdioClient) readLoop(stdout io.Reader) {
	defer close(c.doneCh)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), stdioBufSize)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		// dispatch copies what it needs; the scanner reuses its buffer.
		buf := make([]byte, len(line))
		copy(buf, line)
		c.endpoint.dispatch(buf)
	}
	err := scanner.Err()
	if err == nil {
		err = io.EOF
	}
	c.endpoint.closeWith(err)
}

// Close closes stdin (signalling the server to exit), waits briefly for a clean
// exit, then kills the process group if it overruns. Idempotent.
func (c *stdioClient) Close() error {
	c.closeOnce.Do(func() {
		c.endpoint.closeWith(fmt.Errorf("client closed"))
		_ = c.stdin.Close()

		waitDone := make(chan error, 1)
		go func() { waitDone <- c.cmd.Wait() }()

		select {
		case <-waitDone:
			// clean exit
		case <-time.After(3 * time.Second):
			c.killGroup()
			<-waitDone
		}
		<-c.doneCh
	})
	return nil
}

// killGroup SIGKILLs the subprocess's whole group.
func (c *stdioClient) killGroup() {
	if c.cmd.Process == nil {
		return
	}
	if pgid, err := syscall.Getpgid(c.cmd.Process.Pid); err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	} else {
		_ = c.cmd.Process.Kill()
	}
}

// drainStderr forwards subprocess stderr lines to debug logging.
func drainStderr(r io.Reader, logger Logger) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			logger.Debug("mcp[server stderr]: " + line)
		}
	}
}

// expandArgs expands ${VAR} / $VAR references in each argument, matching what
// env entries and HTTP headers already do.
//
// The reason it is worth having rather than merely consistent: a server that
// keeps state in a directory can be given one per session by writing
// ${KVIT_RUN_ID} into the path. A browser server pointed at a per-session
// profile keeps its cookies across the turn boundary that kills its process,
// and two sessions running at once do not fight over one profile, which a
// persistent profile otherwise forbids.
func expandArgs(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = os.ExpandEnv(a)
	}
	return out
}

// expandEnvEntries expands ${VAR} / $VAR references in the VALUE part of each
// "KEY=VALUE" entry against the current environment.
func expandEnvEntries(entries []string) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		if k, v, ok := strings.Cut(e, "="); ok {
			out[i] = k + "=" + os.ExpandEnv(v)
		} else {
			out[i] = e
		}
	}
	return out
}
