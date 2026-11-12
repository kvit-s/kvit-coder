package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// A tools/call can take as long as whatever is behind it takes. Kvit Works'
// ask_user, for one, waits for a person to answer a question, which can be
// hours. call_timeout therefore limits how long a server may stay silent rather
// than how long a call may take:
//
//  1. MCPTool.Call runs the call under withSilenceTimeout, a context that is
//     cancelled once call_timeout passes with no sign of life from the server.
//  2. proto.CallTool puts a progress token in the request's _meta and records
//     which call it belongs to (progressListeners).
//  3. When the server sends notifications/progress with that token, the
//     transport hands it to proto.handleNotification, which restarts the
//     call's timer.
//
// A server that sends no progress times out after call_timeout, as it would
// with a plain deadline. Whatever ends the wait -- the timer, the turn being
// stopped -- the transport then sends notifications/cancelled naming the
// request, so the server can stop working on it (see rpcEndpoint.abandon).

// progressKey is the context key under which a call's progress callback rides
// from MCPTool.Call, through lazyClient, to proto.CallTool.
type progressKey struct{}

// withProgress returns a context under which a tools/call reports each progress
// notification the server sends for it by calling onProgress.
func withProgress(ctx context.Context, onProgress func()) context.Context {
	return context.WithValue(ctx, progressKey{}, onProgress)
}

// progressFrom returns the progress callback of a context, or nil.
func progressFrom(ctx context.Context) func() {
	f, _ := ctx.Value(progressKey{}).(func())
	return f
}

// silenceError is why a call stopped waiting when its server said nothing for
// call_timeout. It counts as a deadline (errors.Is(err,
// context.DeadlineExceeded)), so code that tells a timeout from a cancellation
// still does.
type silenceError struct{ limit time.Duration }

func (e *silenceError) Error() string {
	return fmt.Sprintf("no reply or progress from the server for %s", e.limit)
}

func (e *silenceError) Unwrap() error { return context.DeadlineExceeded }

// withSilenceTimeout returns a context that is cancelled, with a silenceError as
// its cause, once limit passes without a progress notification for the call
// made under it. Each notification restarts the wait.
func withSilenceTimeout(parent context.Context, limit time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	timer := time.AfterFunc(limit, func() { cancel(&silenceError{limit: limit}) })
	touch := func() {
		// A notification racing the end of the call may re-arm a stopped
		// timer; it then cancels a context that is already cancelled, which
		// does nothing.
		if ctx.Err() == nil {
			timer.Reset(limit)
		}
	}
	return withProgress(ctx, touch), func() {
		timer.Stop()
		cancel(context.Canceled)
	}
}

// cancelReason says in words why a request's context ended, for the reason
// field of notifications/cancelled.
func cancelReason(ctx context.Context) string {
	cause := context.Cause(ctx)
	var silent *silenceError
	switch {
	case errors.As(cause, &silent):
		return silent.Error()
	case errors.Is(cause, context.DeadlineExceeded):
		return "the client's deadline for the request passed"
	default:
		return "the client stopped waiting for the request"
	}
}

// requestMeta is the _meta object of a request's params.
type requestMeta struct {
	ProgressToken string `json:"progressToken,omitempty"`
}

// cancelledParams are the params of notifications/cancelled.
type cancelledParams struct {
	RequestID int64  `json:"requestId"`
	Reason    string `json:"reason,omitempty"`
}

// progressListeners maps the progress token of each call in flight to the
// callback that restarts its timer. Tokens are strings this client makes up;
// the server copies them back unchanged.
type progressListeners struct {
	mu      sync.Mutex
	next    int64
	byToken map[string]func()
}

// add records a callback and returns the token to send with the request.
func (l *progressListeners) add(onProgress func()) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.byToken == nil {
		l.byToken = make(map[string]func())
	}
	l.next++
	token := "call-" + strconv.FormatInt(l.next, 10)
	l.byToken[token] = onProgress
	return token
}

// remove forgets a token once its call has ended.
func (l *progressListeners) remove(token string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.byToken, token)
}

// notify runs the callback named by a notifications/progress. A token this
// client did not issue, or one whose call has ended, is ignored.
func (l *progressListeners) notify(params json.RawMessage) {
	var p struct {
		ProgressToken json.RawMessage `json:"progressToken"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return
	}
	var token string
	if err := json.Unmarshal(p.ProgressToken, &token); err != nil {
		return
	}
	l.mu.Lock()
	onProgress := l.byToken[token]
	l.mu.Unlock()
	if onProgress != nil {
		onProgress()
	}
}
