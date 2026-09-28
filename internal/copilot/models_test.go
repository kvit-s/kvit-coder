package copilot

import "testing"

func TestIsEmbeddingModel(t *testing.T) {
	embed := []ListedModel{
		{ID: "text-embedding-3-small", Name: "Embedding V3 small"},
		{ID: "text-embedding-ada-002", Name: "Embedding V2 Ada"},
		{ID: "text-embedding-3-small-inference", Name: "Embedding V3 small (Inference)"},
		{ID: "some-future-embed-model", Name: "Something Else"},
		{ID: "chat-model", Name: "Embedding Helper"},
		{ID: "chat-model", Endpoints: []string{"/embeddings"}},
	}
	for _, m := range embed {
		if !IsEmbeddingModel(m) {
			t.Errorf("IsEmbeddingModel(%+v) = false, want true", m)
		}
	}
	chat := []ListedModel{
		{ID: "gpt-5-mini", Name: "GPT-5 mini", Endpoints: []string{"/chat/completions"}},
		{ID: "claude-haiku-4.5", Name: "Claude Haiku 4.5", Endpoints: []string{"/v1/messages"}},
		{ID: "gpt-41-copilot", Name: "GPT-4.1 Copilot", Endpoints: []string{"/responses"}},
		{ID: "kimi-k3", Name: "Kimi K3"},
	}
	for _, m := range chat {
		if IsEmbeddingModel(m) {
			t.Errorf("IsEmbeddingModel(%+v) = true, want false", m)
		}
	}
}
