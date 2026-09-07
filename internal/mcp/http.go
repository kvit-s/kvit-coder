package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

// httpClient speaks JSON-RPC 2.0 over the MCP "streamable HTTP" transport: one
// HTTP POST per request. The server may answer with a single JSON object
// (application/json) or a Server-Sent Events stream (text/event-stream); this
// client handles both and tracks the optional Mcp-Session-Id across requests.
type httpClient struct {
	proto
	url    string
	header http.Header
	hc     *http.Client
	logger Logger

	nextID atomic.Int64

	mu        sync.Mutex
	sessionID string
}

// newHTTPClient builds the transport. It does not contact the server; the
// caller invokes Initialize under the startup deadline.
func newHTTPClient(url string, headers []string, logger Logger) (*httpClient, error) {
	if logger == nil {
		logger = nopLogger{}
	}
	if url == "" {
		return nil, fmt.Errorf("http transport requires a url")
	}

	header := http.Header{}
	for _, h := range headers {
		if k, v, ok := strings.Cut(h, "="); ok {
			header.Add(strings.TrimSpace(k), os.ExpandEnv(strings.TrimSpace(v)))
		}
	}

	c := &httpClient{
		url:    url,
		header: header,
		hc:     &http.Client{},
		logger: logger,
	}
	c.proto = proto{conn: c, logger: logger}
	return c, nil
}

// Call performs a single request/response POST.
func (c *httpClient) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	body, err := json.Marshal(rpcRequest{JSONRPC: jsonrpcVersion, ID: &id, Method: method, Params: params})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, ct, err := c.post(ctx, body, method)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case strings.HasPrefix(ct, "text/event-stream"):
		return c.readSSEResponse(resp.Body, id)
	default:
		return c.readJSONResponse(resp.Body, id)
	}
}

// Notify performs a fire-and-forget POST (no response body consumed).
func (c *httpClient) Notify(method string, params any) error {
	body, err := json.Marshal(rpcRequest{JSONRPC: jsonrpcVersion, Method: method, Params: params})
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	resp, _, err := c.post(context.Background(), body, method)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.Body.Close()
}

// post issues the HTTP POST and captures a session id handed back on the first
// response (typically initialize).
func (c *httpClient) post(ctx context.Context, body []byte, method string) (*http.Response, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("build request: %w", err)
	}
	for k, vs := range c.header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if c.negotiated != "" {
		req.Header.Set("MCP-Protocol-Version", c.negotiated)
	}
	c.mu.Lock()
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	c.mu.Unlock()

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", method, err)
	}
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.mu.Lock()
		c.sessionID = sid
		c.mu.Unlock()
	}
	if resp.StatusCode >= 400 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		return nil, "", fmt.Errorf("%s: http %d: %s", method, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return resp, resp.Header.Get("Content-Type"), nil
}

// readJSONResponse parses a single JSON-RPC response object.
func (c *httpClient) readJSONResponse(r io.Reader, id int64) (json.RawMessage, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("empty response")
	}
	return decodeRPCResult(data, id)
}

// readSSEResponse reads SSE events until it finds the JSON-RPC response that
// matches our request id (servers may interleave notifications first).
func (c *httpClient) readSSEResponse(r io.Reader, id int64) (json.RawMessage, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), stdioBufSize)
	var dataLines []string
	flush := func() (json.RawMessage, bool, error) {
		if len(dataLines) == 0 {
			return nil, false, nil
		}
		payload := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		var msg rpcMessage
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			return nil, false, nil // not a JSON-RPC frame; skip
		}
		if msg.ID != nil && *msg.ID == id {
			if msg.Error != nil {
				return nil, true, msg.Error
			}
			return msg.Result, true, nil
		}
		return nil, false, nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" { // event boundary
			if res, done, err := flush(); done || err != nil {
				return res, err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
		// other SSE fields (event:, id:, retry:) are ignored
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read sse: %w", err)
	}
	// stream ended; flush any trailing event
	if res, done, err := flush(); done || err != nil {
		return res, err
	}
	return nil, fmt.Errorf("sse stream ended without a response for id %d", id)
}

// decodeRPCResult extracts the result for the given id from one JSON-RPC object.
func decodeRPCResult(data []byte, id int64) (json.RawMessage, error) {
	var msg rpcMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if msg.Error != nil {
		return nil, msg.Error
	}
	if msg.ID == nil || *msg.ID != id {
		return nil, fmt.Errorf("response id mismatch (want %d)", id)
	}
	return msg.Result, nil
}

// Close releases idle HTTP connections. There is no persistent stream to tear
// down in the request/response model used here.
func (c *httpClient) Close() error {
	c.hc.CloseIdleConnections()
	return nil
}
