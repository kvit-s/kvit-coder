package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// requestBody is the JSON of one request, sent without ever being held in
// memory whole.
//
// A request is small apart from one array, the conversation (messages, or
// input for the Responses API), which grows with the session: 2.4 MB in a
// session of 1,342 messages. json.Marshal built all of it in a buffer that
// doubles as it grows and then copied the result, for every request of a
// turn. Here only the rest of the request is marshalled, and the array's
// elements are encoded one at a time as the transport reads the body, so the
// largest piece in memory is one message. The bytes are exactly what
// json.Marshal of the whole request produces, so what the server renders,
// and its prompt cache, are unchanged.
type requestBody struct {
	head []byte // the request up to and including the array's '['
	tail []byte // the request from the array's ']' to the end
	n    int
	elem func(i int) any
	size int64
}

// newRequestBody prepares the body for request, whose top-level field key
// holds an empty, non-nil slice when n is above zero; the n elements that
// belong there come from elem. elem should return a pointer to the element, as json.Marshal of the
// slice would use a pointer-receiver MarshalJSON. Every element is encoded
// once here to learn the length, so the request carries a Content-Length,
// and an element that cannot be encoded fails here rather than mid-send.
func newRequestBody(request any, key string, n int, elem func(i int) any) (*requestBody, error) {
	frame, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		// Nothing to stream. The caller leaves the field as it was, which
		// may be nil and marshal as null, so the frame is the whole body.
		return &requestBody{head: frame, size: int64(len(frame))}, nil
	}
	head, tail, err := splitAtEmptyArray(frame, key)
	if err != nil {
		return nil, err
	}
	b := &requestBody{head: head, tail: tail, n: n, elem: elem}
	b.size = int64(len(head) + len(tail))
	var enc elementEncoder
	for i := 0; i < n; i++ {
		data, err := enc.encode(elem(i), i > 0)
		if err != nil {
			return nil, err
		}
		b.size += int64(len(data))
	}
	return b, nil
}

// Len is the body's length in bytes.
func (b *requestBody) Len() int64 { return b.size }

// Open returns a reader of the body from its start. Each attempt of a
// request opens its own.
func (b *requestBody) Open() io.ReadCloser { return &requestBodyReader{b: b} }

// Bytes encodes the whole body in memory, for tests.
func (b *requestBody) Bytes() ([]byte, error) { return io.ReadAll(b.Open()) }

// splitAtEmptyArray finds the empty array in frame's top-level field key and
// returns frame up to and including its '[' and from its ']'. Walking the
// top level with a decoder, rather than searching the text, cannot be misled
// by a field of the same name nested deeper, such as in a tool's schema.
func splitAtEmptyArray(frame []byte, key string) ([]byte, []byte, error) {
	dec := json.NewDecoder(bytes.NewReader(frame))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, nil, fmt.Errorf("request is not a JSON object")
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		if name, _ := tok.(string); name != key {
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return nil, nil, err
			}
			continue
		}
		if tok, err := dec.Token(); err != nil || tok != json.Delim('[') {
			return nil, nil, fmt.Errorf("request field %q is not an array", key)
		}
		start := dec.InputOffset()
		if tok, err := dec.Token(); err != nil || tok != json.Delim(']') {
			return nil, nil, fmt.Errorf("request field %q is not empty", key)
		}
		end := dec.InputOffset() - 1
		return frame[:start], frame[end:], nil
	}
	return nil, nil, fmt.Errorf("request has no field %q", key)
}

// elementEncoder encodes one array element into a buffer it reuses.
type elementEncoder struct {
	buf bytes.Buffer
	enc *json.Encoder
}

// encode returns v's JSON, preceded by a comma when it follows another
// element. The slice is valid until the next call.
func (e *elementEncoder) encode(v any, comma bool) ([]byte, error) {
	if e.enc == nil {
		e.enc = json.NewEncoder(&e.buf)
	}
	e.buf.Reset()
	if comma {
		e.buf.WriteByte(',')
	}
	if err := e.enc.Encode(v); err != nil {
		return nil, err
	}
	// Encode ends each value with a newline, which json.Marshal does not.
	out := e.buf.Bytes()
	return out[:len(out)-1], nil
}

// requestBodyReader produces the body in order: the head, then each element
// as it is reached, then the tail.
type requestBodyReader struct {
	b       *requestBody
	started bool
	next    int // the next element to encode
	ended   bool
	pending []byte
	enc     elementEncoder
	err     error
}

func (r *requestBodyReader) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		if len(r.pending) == 0 && !r.fill() {
			break
		}
		c := copy(p[n:], r.pending)
		r.pending = r.pending[c:]
		n += c
	}
	if n > 0 {
		return n, nil
	}
	if r.err != nil {
		return 0, r.err
	}
	return 0, io.EOF
}

// fill sets pending to the next piece of the body, and reports false once
// there is none or an element failed to encode.
func (r *requestBodyReader) fill() bool {
	switch {
	case r.err != nil || r.ended:
		return false
	case !r.started:
		r.started = true
		r.pending = r.b.head
	case r.next < r.b.n:
		data, err := r.enc.encode(r.b.elem(r.next), r.next > 0)
		if err != nil {
			r.err = err
			return false
		}
		r.next++
		r.pending = data
	default:
		r.ended = true
		r.pending = r.b.tail
	}
	return true
}

func (r *requestBodyReader) Close() error { return nil }
