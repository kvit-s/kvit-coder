// Package acp is the wire side of the Agent Client Protocol: JSON-RPC 2.0
// messages, one per line, read from the client on standard input and written
// to it on standard output. What the messages mean is the caller's
// (cmd/kvit-coder/acp.go); this package only carries them.
//
// Requests from the client are handled each on its own goroutine, so a
// session/cancel, or the answer to a question this side asked, is read while
// a prompt is still running. Responses and notifications are written whole,
// one at a time.
package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
)

// The JSON-RPC error codes this side sends.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Error is a JSON-RPC error, sent as the answer to a request or received as
// one.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *Error) Error() string { return e.Message }

// Errorf makes an error with a code.
func Errorf(code int, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Handler answers what the client sends. HandleRequest returns the result to
// send back, or an error; an *Error keeps its code and any other error is
// sent as an internal error with its text. HandleNotification has nothing to
// answer.
type Handler interface {
	HandleRequest(ctx context.Context, method string, params json.RawMessage) (any, error)
	HandleNotification(method string, params json.RawMessage)
}

type message struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *Error           `json:"error,omitempty"`
}

type reply struct {
	result json.RawMessage
	err    *Error
}

// Conn is one connection to a client.
type Conn struct {
	r       io.Reader
	w       io.Writer
	handler Handler

	writeMu sync.Mutex

	pendingMu sync.Mutex
	nextID    int64
	pending   map[string]chan reply
	closed    bool
}

// NewConn makes a connection reading r and writing w.
func NewConn(r io.Reader, w io.Writer, h Handler) *Conn {
	return &Conn{r: r, w: w, handler: h, pending: map[string]chan reply{}}
}

// Serve reads messages until the input ends or ctx is done. Requests still
// being handled then are told through their context and waited for, so a
// prompt that was running writes down what it did before Serve returns.
func (c *Conn) Serve(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var handlers sync.WaitGroup
	defer func() {
		cancel()
		c.failPending()
		handlers.Wait()
	}()

	lines := make(chan []byte)
	readErr := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(c.r)
		// A prompt with pictures in it is one line of base64.
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024*1024)
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			select {
			case lines <- line:
			case <-ctx.Done():
				return
			}
		}
		readErr <- scanner.Err()
	}()

	for {
		var line []byte
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-readErr:
			return err
		case line = <-lines:
		}
		if len(line) == 0 {
			continue
		}
		var msg message
		if err := json.Unmarshal(line, &msg); err != nil {
			_ = c.send(message{ID: rawNull(), Error: Errorf(CodeParseError, "not a JSON-RPC message: %v", err)})
			continue
		}
		switch {
		case msg.Method != "" && msg.ID != nil:
			handlers.Add(1)
			go func(msg message) {
				defer handlers.Done()
				c.answer(ctx, msg)
			}(msg)
		case msg.Method != "":
			c.handler.HandleNotification(msg.Method, msg.Params)
		case msg.ID != nil:
			c.deliver(msg)
		}
	}
}

func rawNull() *json.RawMessage {
	null := json.RawMessage("null")
	return &null
}

func (c *Conn) answer(ctx context.Context, msg message) {
	result, err := c.handler.HandleRequest(ctx, msg.Method, msg.Params)
	out := message{ID: msg.ID}
	if err != nil {
		var rpcErr *Error
		if !errors.As(err, &rpcErr) {
			rpcErr = &Error{Code: CodeInternalError, Message: err.Error()}
		}
		out.Error = rpcErr
	} else {
		if result == nil {
			result = struct{}{}
		}
		data, merr := json.Marshal(result)
		if merr != nil {
			out.Error = Errorf(CodeInternalError, "cannot encode the result: %v", merr)
		} else {
			out.Result = data
		}
	}
	_ = c.send(out)
}

func (c *Conn) send(msg message) error {
	msg.JSONRPC = "2.0"
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.w.Write(data)
	return err
}

// Notify sends a notification.
func (c *Conn) Notify(method string, params any) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.send(message{Method: method, Params: data})
}

// Call sends a request and waits for its answer, which is decoded into
// result when that is not nil. It gives up when ctx is done.
func (c *Conn) Call(ctx context.Context, method string, params any, result any) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	c.pendingMu.Lock()
	if c.closed {
		c.pendingMu.Unlock()
		return errors.New("the connection is closed")
	}
	c.nextID++
	id := strconv.FormatInt(c.nextID, 10)
	ch := make(chan reply, 1)
	c.pending[id] = ch
	c.pendingMu.Unlock()
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	rawID := json.RawMessage(id)
	if err := c.send(message{ID: &rawID, Method: method, Params: data}); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return r.err
		}
		if result != nil && len(r.result) > 0 {
			return json.Unmarshal(r.result, result)
		}
		return nil
	}
}

func (c *Conn) deliver(msg message) {
	id := string(*msg.ID)
	// An id this side sent is a number; one echoed back as a string still
	// names the same request.
	if unquoted, err := strconv.Unquote(id); err == nil {
		id = unquoted
	}
	c.pendingMu.Lock()
	ch := c.pending[id]
	c.pendingMu.Unlock()
	if ch == nil {
		return
	}
	ch <- reply{result: msg.Result, err: msg.Error}
}

// failPending answers every call still waiting with an error, since no
// answer can arrive once the input has ended.
func (c *Conn) failPending() {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	c.closed = true
	for id, ch := range c.pending {
		select {
		case ch <- reply{err: Errorf(CodeInternalError, "the client went away")}:
		default:
		}
		delete(c.pending, id)
	}
}
