package benchmark

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Sandbox describes the resolved sandbox configuration for a thinkbench run.
type Sandbox struct {
	Tool       string   // "bwrap", "firejail", or "none"
	ExecPrefix []string // argv prepended before "sh -c <cmd>" ({workdir} substituted at run time)
}

// thinkbenchEnv carries the resolved uv + sandbox environment for the run.
type thinkbenchEnv struct {
	UVDir     string   // abs path to the shared uv venv
	UVBin     string   // abs path to <uv>/bin
	UVPy      string   // abs path to <uv>/bin/python
	PyPrefix  string   // base interpreter prefix to bind into the sandbox ("" if it's a system path)
	InjectEnv []string // env entries injected into agent run_command + grader
	Sandbox   thinkbenchSandbox
}

type thinkbenchSandbox = Sandbox

// ensureUV verifies the uv binary is on PATH.
func ensureUV() error {
	if _, err := exec.LookPath("uv"); err != nil {
		return fmt.Errorf("`uv` not found on PATH. Install it: https://docs.astral.sh/uv/getting-started/installation/ (e.g. `curl -LsSf https://astral.sh/uv/install.sh | sh`)")
	}
	return nil
}

// provisionUV creates (idempotently) a shared uv venv at uvDir pinned to
// pyVersion, and returns the injected environment that makes python/python3/pip
// resolve into it purely via PATH (no outside path appears in any command).
func provisionUV(ctx context.Context, uvDir, pyVersion string, out io.Writer) (*thinkbenchEnv, error) {
	if err := ensureUV(); err != nil {
		return nil, err
	}

	absUV, err := filepath.Abs(uvDir)
	if err != nil {
		return nil, err
	}
	uvBin := filepath.Join(absUV, "bin")
	uvPy := filepath.Join(uvBin, "python")
	uvPip := filepath.Join(uvBin, "pip")

	// (Re)create the venv if it lacks an interpreter or pip. The venv is seeded
	// with pip/setuptools so agents can `pip install` their own test deps.
	_, pyErr := os.Stat(uvPy)
	_, pipErr := os.Stat(uvPip)
	if pyErr != nil || pipErr != nil {
		if err := os.MkdirAll(filepath.Dir(absUV), 0755); err != nil {
			return nil, err
		}
		fmt.Fprintf(out, "Provisioning uv environment at %s (python %s, seeded)...\n", absUV, pyVersion)
		cmd := exec.CommandContext(ctx, "uv", "venv", "--seed", "--python", pyVersion, absUV)
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("uv venv failed: %w", err)
		}
	}

	// Verify the interpreter works.
	if _, err := os.Stat(uvPy); err != nil {
		return nil, fmt.Errorf("uv venv did not produce an interpreter at %s", uvPy)
	}

	// Resolve the base interpreter the venv links to. uv often uses a managed
	// standalone CPython under ~/.local/share/uv/python/<...>, which lives outside
	// the suite/workspace and must be bind-mounted (read-only) into the sandbox or
	// the venv's python symlink dangles and falls back to system python.
	pyPrefix := ""
	if real, err := filepath.EvalSymlinks(uvPy); err == nil {
		// real == <prefix>/bin/pythonX → prefix is two levels up.
		prefix := filepath.Dir(filepath.Dir(real))
		if !isSystemBoundPath(real) && !strings.HasPrefix(real, absUV+string(os.PathSeparator)) {
			pyPrefix = prefix
		}
	}

	injected := []string{
		"VIRTUAL_ENV=" + absUV,
		"UV_PROJECT_ENVIRONMENT=" + absUV,
		"PATH=" + uvBin + string(os.PathListSeparator) + os.Getenv("PATH"),
	}

	return &thinkbenchEnv{
		UVDir:     absUV,
		UVBin:     uvBin,
		UVPy:      uvPy,
		PyPrefix:  pyPrefix,
		InjectEnv: injected,
	}, nil
}

// isSystemBoundPath reports whether a path lives under a system directory that
// the sandbox already binds read-only (so it needs no extra bind).
func isSystemBoundPath(p string) bool {
	for _, top := range []string{"/usr/", "/bin/", "/sbin/", "/lib/", "/lib32/", "/lib64/", "/opt/"} {
		if strings.HasPrefix(p, top) {
			return true
		}
	}
	return false
}

// installPipPackages installs the given packages into the shared uv env (once
// per invocation, at preflight) so agents can use common test deps (pytest)
// without reaching outside the sandbox to locate them. It is best-effort: a
// failure (e.g. offline) is warned and the run continues — agents fall back to
// the stdlib (unittest), and the held-out grader is stdlib-only regardless.
func installPipPackages(ctx context.Context, env *thinkbenchEnv, pkgs []string, out io.Writer) {
	if len(pkgs) == 0 {
		return
	}

	// Skip if every package already imports in the env (avoids a network hit on
	// resume runs). Cheap probe via the env interpreter.
	probe := "import importlib.util,sys; sys.exit(0 if all(importlib.util.find_spec(m) for m in sys.argv[1:]) else 1)"
	check := exec.CommandContext(ctx, env.UVPy, append([]string{"-c", probe}, importNames(pkgs)...)...)
	check.Env = append(os.Environ(), env.InjectEnv...)
	if check.Run() == nil {
		return // all present
	}

	fmt.Fprintf(out, "Installing test deps into uv env: %s\n", strings.Join(pkgs, " "))
	args := append([]string{"pip", "install", "--python", env.UVPy, "--quiet"}, pkgs...)
	cmd := exec.CommandContext(ctx, "uv", args...)
	cmd.Env = append(os.Environ(), env.InjectEnv...)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(out, "⚠️  could not pre-install %v (%v); agents will fall back to stdlib unittest\n", pkgs, err)
	}
}

// importNames maps pip package names to their import module name for the
// presence probe (only the common ones that differ need special-casing).
func importNames(pkgs []string) []string {
	special := map[string]string{
		"pytest-cov":     "pytest_cov",
		"pyyaml":         "yaml",
		"beautifulsoup4": "bs4",
	}
	out := make([]string, len(pkgs))
	for i, p := range pkgs {
		// Strip any version specifier (pytest==8.0 → pytest).
		name := p
		for _, sep := range []string{"==", ">=", "<=", "~=", ">", "<", "["} {
			if idx := strings.Index(name, sep); idx >= 0 {
				name = name[:idx]
			}
		}
		name = strings.TrimSpace(name)
		if mod, ok := special[strings.ToLower(name)]; ok {
			out[i] = mod
		} else {
			out[i] = strings.ReplaceAll(name, "-", "_")
		}
	}
	return out
}

// resolveSandbox selects and builds the OS sandbox wrapper per the configured
// mode. modes: "auto" (bwrap→firejail→none), "bwrap", "firejail", "none",
// "require" (auto but hard-fail if none found). uvDir is bind-mounted read-only;
// workspace is the (single) per-run workspace bound read-write.
func resolveSandbox(mode string, env *thinkbenchEnv, workspace string, out io.Writer) (Sandbox, error) {
	bwrap, _ := exec.LookPath("bwrap")
	firejail, _ := exec.LookPath("firejail")

	pick := func(tool string) (Sandbox, error) {
		switch tool {
		case "bwrap":
			return Sandbox{Tool: "bwrap", ExecPrefix: buildBwrapPrefix(bwrap, env.UVDir, env.PyPrefix, workspace)}, nil
		case "firejail":
			return Sandbox{Tool: "firejail", ExecPrefix: buildFirejailPrefix(firejail, workspace)}, nil
		default:
			return Sandbox{Tool: "none"}, nil
		}
	}

	switch mode {
	case "none":
		fmt.Fprintf(out, "Sandbox: disabled (sandbox=none). Arbitrary-shell reads are confined only by path_safety_mode=block (structured tools).\n")
		return Sandbox{Tool: "none"}, nil
	case "bwrap":
		if bwrap == "" {
			return Sandbox{}, fmt.Errorf("sandbox=bwrap but bwrap not found on PATH")
		}
		return pick("bwrap")
	case "firejail":
		if firejail == "" {
			return Sandbox{}, fmt.Errorf("sandbox=firejail but firejail not found on PATH")
		}
		return pick("firejail")
	case "auto", "require":
		if bwrap != "" {
			fmt.Fprintf(out, "Sandbox: bubblewrap (%s)\n", bwrap)
			return pick("bwrap")
		}
		if firejail != "" {
			fmt.Fprintf(out, "Sandbox: firejail (%s)\n", firejail)
			return pick("firejail")
		}
		if mode == "require" {
			return Sandbox{}, fmt.Errorf("sandbox=require but neither bwrap nor firejail is available")
		}
		fmt.Fprintf(out, "⚠️  Sandbox: NONE — no bwrap/firejail found. Arbitrary-shell reads are NOT airtight (structured file tools are still confined by path_safety_mode=block). Install bubblewrap for a hard guarantee, or set thinkbench.sandbox=require to fail instead.\n")
		return Sandbox{Tool: "none"}, nil
	default:
		return Sandbox{}, fmt.Errorf("unknown thinkbench.sandbox=%q (want auto|bwrap|firejail|none|require)", mode)
	}
}

// buildBwrapPrefix builds a read-confining bubblewrap argv. It binds standard
// system directories (read-only, symlink-aware for merged-/usr hosts), the uv
// venv's base interpreter (read-only), the venv itself (read-write, so agents
// can `pip install` their own test deps), and the run workspace (read-write) —
// and nothing else. The repo/suite (held-out graders, references, other tasks)
// is NOT bound, so even arbitrary shell cannot read it.
func buildBwrapPrefix(bwrap, uvDir, pyPrefix, workspace string) []string {
	args := []string{bwrap, "--die-with-parent", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp"}

	// System directories needed to run the interpreter, read-only.
	for _, top := range []string{"/usr", "/bin", "/sbin", "/lib", "/lib32", "/lib64", "/etc", "/opt"} {
		args = append(args, topDirBindArgs(top)...)
	}

	// Base interpreter (uv-managed standalone CPython), read-only — required so
	// the venv's python symlink resolves inside the sandbox.
	if pyPrefix != "" {
		args = append(args, "--ro-bind", pyPrefix, pyPrefix)
	}

	// uv env read-write (so `pip install` into the venv works); workspace
	// read-write. The venv is shared, so per-run installs persist across runs —
	// harmless for scoring since the held-out grader imports only the agent's
	// package + stdlib.
	args = append(args, "--bind", uvDir, uvDir)
	args = append(args, "--bind", workspace, workspace)
	args = append(args, "--chdir", "{workdir}")
	args = append(args, "--")
	return args
}

// topDirBindArgs returns bwrap args for a top-level directory, recreating
// symlinks as symlinks (merged-/usr layout) and binding real dirs read-only.
// Returns nil if the path does not exist.
func topDirBindArgs(path string) []string {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return nil
		}
		return []string{"--symlink", target, path}
	}
	return []string{"--ro-bind", path, path}
}

// buildFirejailPrefix builds a firejail argv that whitelists only the workspace
// (and the uv env, reachable read-only). firejail's --private/whitelist model is
// coarser than bwrap; this is a best-effort fallback.
func buildFirejailPrefix(firejail, workspace string) []string {
	return []string{
		firejail,
		"--quiet",
		"--noprofile",
		"--private=" + workspace,
		"--net=none",
		"--",
	}
}

// smokeTestSandbox verifies the sandbox actually denies reads outside the run
// workspace. It runs, through the sandbox, a command that tries to read a path
// known to be outside the workspace, and asserts the read fails. With Tool=none
// it is a no-op. Returns an error if the sandbox is misconfigured (read allowed).
func smokeTestSandbox(ctx context.Context, sb Sandbox, env *thinkbenchEnv, workspace, outsidePath string, out io.Writer) error {
	if sb.Tool == "none" || len(sb.ExecPrefix) == 0 {
		return nil
	}

	// Build argv: prefix (with {workdir} substituted) + sh -c <probe>.
	probe := fmt.Sprintf("cat %s 2>/dev/null; echo EXIT=$?", shellEscape(outsidePath))
	args := make([]string, 0, len(sb.ExecPrefix)+3)
	for _, a := range sb.ExecPrefix {
		args = append(args, strings.ReplaceAll(a, "{workdir}", workspace))
	}
	args = append(args, "sh", "-c", probe)

	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, args[0], args[1:]...)
	cmd.Dir = workspace
	if len(env.InjectEnv) > 0 {
		cmd.Env = append(os.Environ(), env.InjectEnv...)
	}
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		// The sandbox tool itself failed to launch (e.g. bwrap can't create
		// namespaces). Surface as a configuration error.
		return fmt.Errorf("sandbox smoke test failed to run (%s): %v\noutput: %s", sb.Tool, err, string(outBytes))
	}

	output := string(outBytes)
	// The probe must NOT have produced the file's contents and the cat must have
	// returned non-zero. We seeded a sentinel into outsidePath; assert it's absent.
	if strings.Contains(output, thinkbenchSandboxSentinel) || strings.Contains(output, "EXIT=0") {
		return fmt.Errorf("sandbox %s did NOT deny a read outside the workspace (%s); refusing to run. Probe output: %s", sb.Tool, outsidePath, output)
	}
	fmt.Fprintf(out, "Sandbox smoke test passed: read of %s denied inside %s.\n", outsidePath, sb.Tool)
	return nil
}

// thinkbenchSandboxSentinel is written into the probe target so the smoke test
// can prove the read was denied (its absence in output == denial).
const thinkbenchSandboxSentinel = "THINKBENCH_SANDBOX_LEAK_SENTINEL"

// writeSandboxProbeFile writes a sentinel file outside the workspace for the
// smoke test, returning its path. The caller removes it afterwards.
func writeSandboxProbeFile(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, ".thinkbench-sandbox-probe")
	if err := os.WriteFile(p, []byte(thinkbenchSandboxSentinel+"\n"), 0644); err != nil {
		return "", err
	}
	return p, nil
}
