package modelsetup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// listTimeout bounds one GET /models. A local server answers in
// milliseconds, a hosted gateway in well under a second.
const listTimeout = 20 * time.Second

// ListedModel is one model an endpoint says it serves.
type ListedModel struct {
	ID string
	// Context is the context size the endpoint reports, or 0: vLLM puts
	// max_model_len in its model list, and for llama.cpp ServerContext asks
	// /props. Hosted gateways report nothing here; models.dev has it.
	Context int
}

// ListModels asks an OpenAI-compatible endpoint which models it serves:
// GET <baseURL>/models, the request llama.cpp, vLLM, Ollama, LM Studio and
// the hosted gateways all answer. The list is sorted by id.
func ListModels(ctx context.Context, client *http.Client, baseURL, key string, headers map[string]string) ([]ListedModel, error) {
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()
	url := strings.TrimRight(baseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %d: %s", url, resp.StatusCode, shorten(string(body), 300))
	}
	var list struct {
		Data []struct {
			ID          string `json:"id"`
			MaxModelLen int    `json:"max_model_len"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("%s did not answer with a model list: %w", url, err)
	}
	var models []ListedModel
	seen := map[string]bool{}
	for _, m := range list.Data {
		if m.ID == "" || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		models = append(models, ListedModel{ID: m.ID, Context: m.MaxModelLen})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, nil
}

// ServerContext asks a llama.cpp server for the context size it was started
// with, from GET /props at the address without its /v1. Any other server, or
// any failure, gives 0, and the row is written without a context size.
func ServerContext(ctx context.Context, client *http.Client, baseURL string) int {
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	url := strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1") + "/props"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}
	var props struct {
		Settings struct {
			NCtx int `json:"n_ctx"`
		} `json:"default_generation_settings"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&props); err != nil {
		return 0
	}
	return props.Settings.NCtx
}

// shorten cuts s to n bytes for an error message, on a rune boundary.
func shorten(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && (s[cut]&0xC0) == 0x80 {
		cut--
	}
	return s[:cut] + "…"
}
