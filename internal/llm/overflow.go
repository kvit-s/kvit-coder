package llm

import (
	"errors"
	"net/http"
	"strings"
)

// overflowPhrases are the ways endpoints say a request did not fit the model's
// context window. OpenAI uses the code context_length_exceeded and "maximum
// context length"; Anthropic says "prompt is too long"; llama.cpp, vLLM and
// the gateways in front of them use the rest.
var overflowPhrases = []string{
	"context_length_exceeded",
	"maximum context length",
	"prompt is too long",
	"context window",
	"context length",
	"exceeds the context",
	"too many tokens",
	"input is too long",
	"request too large",
	"reduce the length",
}

// IsContextOverflow reports whether err is the endpoint refusing a request
// because it is longer than the model's context window. Any other refusal — a
// malformed schema, a bad image, a refused key — is not, which matters because
// the answer to an overflow is to compact the conversation, and compacting it
// would not fix any of those.
func IsContextOverflow(err error) bool {
	if err == nil {
		return false
	}
	var status *StatusError
	if errors.As(err, &status) {
		if status.Code == http.StatusRequestEntityTooLarge {
			return true
		}
		if status.Code != http.StatusBadRequest {
			return false
		}
		return mentionsOverflow(string(status.Body))
	}
	return mentionsOverflow(err.Error())
}

func mentionsOverflow(text string) bool {
	text = strings.ToLower(text)
	for _, phrase := range overflowPhrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}
