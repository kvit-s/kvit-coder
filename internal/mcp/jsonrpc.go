package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// jsonrpcVersion is the only JSON-RPC version MCP uses.
const jsonrpcVersion = "2.0"

// rpcRequest is an outgoing JSON-RPC request or notification. A notification
// omits id (ID == nil).
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int64 `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// rpcError is a JSON-RPC error object.
type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// rpcMessage is any incoming JSON-RPC message. Responses carry id+result/error;
// notifications and server→client requests carry method (and maybe id).
type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcResponse is delivered to a waiting caller.
type rpcResponse struct {
	result json.RawMessage
	err    *rpcError
}

// rpcEndpoint correlates JSON-RPC requests with responses over a single
// transport. A transport supplies writeFn (serialize+send one message) and
// feeds every received message into dispatch. The endpoint is transport-agnostic
// and safe for concurrent use.
type rpcEndpoint struct {
	writeFn func([]byte) error
	logger  Logger
	// onNotification, when set, receives every notification the server sends.
	// It runs on the transport's reader, so it must not block. Set it before
	// the first message can arrive.
	onNotification func(method string, params json.RawMessage)

	mu       sync.Mutex
	nextID   int64
	pending  map[int64]chan rpcResponse
	closed   bool
	closeErr error
}

func newRPCEndpoint(writeFn func([]byte) error, logger Logger) *rpcEndpoint {
	if logger == nil {
		logger = nopLogger{}
	}
	return &rpcEndpoint{
		writeFn: writeFn,
		logger:  logger,
		pending: make(map[int64]chan rpcResponse),
	}
}

// Call sends a request and blocks until the matching response arrives, the
// context is cancelled, or the endpoint is closed. A request given up on because
// its context ended is cancelled with the server (see abandon), and the error
// returned is the context's cause.
func (e *rpcEndpoint) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, fmt.Errorf("connection closed: %w", e.closeErr)
	}
	e.nextID++
	id := e.nextID
	ch := make(chan rpcResponse, 1)
	e.pending[id] = ch
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.pending, id)
		e.mu.Unlock()
	}()

	payload, err := json.Marshal(rpcRequest{
		JSONRPC: jsonrpcVersion,
		ID:      &id,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if err := e.writeFn(payload); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	select {
	case resp := <-ch:
		return resp.unwrap()
	case <-ctx.Done():
		// An answer that came in as the context ended is still the answer.
		select {
		case resp := <-ch:
			return resp.unwrap()
		default:
		}
		e.abandon(id, method, ctx)
		return nil, context.Cause(ctx)
	}
}

func (r rpcResponse) unwrap() (json.RawMessage, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.result, nil
}

// abandon tells the server that the client no longer waits for request id, so
// the server can stop the work and, for a question shown to a person, take the
// question down. Without it the server keeps going and its answer goes to a
// request nobody is waiting for. The specification forbids cancelling
// initialize, so that request is left alone.
func (e *rpcEndpoint) abandon(id int64, method string, ctx context.Context) {
	if method == "initialize" {
		return
	}
	err := e.Notify("notifications/cancelled", cancelledParams{RequestID: id, Reason: cancelReason(ctx)})
	if err != nil {
		e.logger.Debug(fmt.Sprintf("mcp: could not cancel %s request %d: %v", method, id, err))
	}
}

// Notify sends a fire-and-forget notification (no id, no response expected).
func (e *rpcEndpoint) Notify(method string, params any) error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return fmt.Errorf("connection closed: %w", e.closeErr)
	}
	e.mu.Unlock()

	payload, err := json.Marshal(rpcRequest{
		JSONRPC: jsonrpcVersion,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	return e.writeFn(payload)
}

// dispatch routes one received message. Responses (id present, no method) are
// delivered to the waiting caller; notifications and server→client requests are
// logged and otherwise ignored (this client advertises no capabilities that
// would require answering server requests).
func (e *rpcEndpoint) dispatch(raw []byte) {
	var msg rpcMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		e.logger.Warn(fmt.Sprintf("mcp: dropping unparseable message: %v", err))
		return
	}

	// A response has an id and no method.
	if msg.ID != nil && msg.Method == "" {
		e.mu.Lock()
		ch, ok := e.pending[*msg.ID]
		e.mu.Unlock()
		if ok {
			deliver(ch, rpcResponse{result: msg.Result, err: msg.Error})
		}
		return
	}

	// A notification goes to the protocol layer, which acts on progress.
	if msg.ID == nil && msg.Method != "" && e.onNotification != nil {
		e.onNotification(msg.Method, msg.Params)
		return
	}

	// Otherwise it's a server→client request. This client advertises no
	// capability a server could ask it to use; just record it for debugging.
	if msg.Method != "" {
		e.logger.Debug(fmt.Sprintf("mcp: ignoring server message method=%s", msg.Method))
	}
}

// deliver hands a response to a waiting Call without blocking. Each channel
// holds one response; when it is already full -- a response arrived and then
// the connection closed before Call read it, or Call gave up -- the later one
// is dropped rather than left to block the reader forever.
func deliver(ch chan rpcResponse, resp rpcResponse) {
	select {
	case ch <- resp:
	default:
	}
}

// closeWith fails all pending calls and marks the endpoint closed. Idempotent.
func (e *rpcEndpoint) closeWith(err error) {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	e.closeErr = err
	pending := e.pending
	e.pending = make(map[int64]chan rpcResponse)
	e.mu.Unlock()

	for _, ch := range pending {
		deliver(ch, rpcResponse{err: &rpcError{Code: -1, Message: fmt.Sprintf("connection closed: %v", err)}})
	}
}
