package modelsetup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// models.dev is a public catalog of model metadata kept by the OpenCode
// project: one JSON file listing, per provider, its endpoint and key variable,
// and per model its context size, whether it reasons and at which effort
// levels, its prices, and which client library talks to it. That last field
// is what tells the protocols apart, which is the fact a person setting up
// OpenCode by hand otherwise has to look up model by model.

// CatalogURL is where the catalog is fetched from. Tests point it elsewhere.
var CatalogURL = "https://models.dev/api.json"

// CatalogTTL is how long a fetched catalog is used before it is fetched
// again. models.dev changes a few times a week; a day keeps a new model a day
// away at most.
const CatalogTTL = 24 * time.Hour

// catalogTimeout bounds one fetch. The file is about 5 MB.
const catalogTimeout = 60 * time.Second

// Catalog is the part of models.dev kvit-coder-ui uses: the providers in
// Providers, keyed by their models.dev id.
type Catalog struct {
	FetchedAt time.Time                  `json:"fetched_at"`
	Providers map[string]CatalogProvider `json:"providers"`
}

// CatalogProvider is one models.dev provider.
type CatalogProvider struct {
	ID     string                  `json:"id"`
	Name   string                  `json:"name"`
	NPM    string                  `json:"npm"`
	API    string                  `json:"api"`
	Env    []string                `json:"env"`
	Models map[string]CatalogModel `json:"models"`
}

// CatalogModel is one model of a models.dev provider.
type CatalogModel struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Reasoning        bool              `json:"reasoning"`
	ReasoningOptions []ReasoningOption `json:"reasoning_options,omitempty"`
	ToolCall         bool              `json:"tool_call"`
	Status           string            `json:"status,omitempty"`
	Limit            struct {
		Context int `json:"context"`
		Output  int `json:"output"`
	} `json:"limit"`
	Cost *struct {
		Input  float64 `json:"input"`
		Output float64 `json:"output"`
	} `json:"cost,omitempty"`
	// Provider overrides the provider's client library for this model, which
	// is how one gateway serving three protocols is described.
	Provider *struct {
		NPM string `json:"npm"`
	} `json:"provider,omitempty"`
}

// ReasoningOption is one way a model's reasoning is controlled. Only the
// "effort" kind, with its list of accepted levels, maps onto kvit-coder's
// efforts; models.dev also has "toggle" and "budget_tokens".
type ReasoningOption struct {
	Type   string   `json:"type"`
	Values []string `json:"values,omitempty"`
}

// Model looks a model up; nil when the catalog does not describe it.
func (c *Catalog) Model(providerID, modelID string) *CatalogModel {
	if c == nil {
		return nil
	}
	p, ok := c.Providers[providerID]
	if !ok {
		return nil
	}
	m, ok := p.Models[modelID]
	if !ok {
		return nil
	}
	return &m
}

// ProviderNPM is the client library a provider uses for models that name
// none of their own.
func (c *Catalog) ProviderNPM(providerID string) string {
	if c == nil {
		return ""
	}
	return c.Providers[providerID].NPM
}

// LoadCatalog returns the catalog from the cache in dir when it is younger
// than CatalogTTL, and fetches it otherwise. When the fetch fails and an older
// cache exists, that cache is returned with a warning; when there is no cache
// either, the error is returned and :setup carries on without the catalog,
// asking for what it would have said.
func LoadCatalog(ctx context.Context, client *http.Client, dir string, now time.Time) (*Catalog, string, error) {
	path := filepath.Join(dir, "models-dev.json")
	cached, cacheErr := readCatalogCache(path)
	if cached != nil && now.Sub(cached.FetchedAt) < CatalogTTL && cached.covers(CatalogIDs()) {
		return cached, "", nil
	}
	fresh, err := FetchCatalog(ctx, client, now)
	if err != nil {
		if cached != nil {
			return cached, fmt.Sprintf("could not refresh the models.dev catalog (%v); using the copy from %s",
				err, cached.FetchedAt.Local().Format("2 Jan 15:04")), nil
		}
		if cacheErr != nil {
			err = fmt.Errorf("%w (and the cached copy could not be read: %v)", err, cacheErr)
		}
		return nil, "", err
	}
	if err := writeCatalogCache(path, fresh); err != nil {
		return fresh, fmt.Sprintf("could not save the models.dev catalog to %s: %v", path, err), nil
	}
	return fresh, "", nil
}

// covers reports whether the cache holds every provider asked for. A newer
// kvit-coder-ui may know a provider an older one did not keep.
func (c *Catalog) covers(ids []string) bool {
	for _, id := range ids {
		if _, ok := c.Providers[id]; !ok {
			return false
		}
	}
	return true
}

// FetchCatalog downloads models.dev and keeps the providers in Providers.
func FetchCatalog(ctx context.Context, client *http.Client, now time.Time) (*Catalog, error) {
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, catalogTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, CatalogURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "kvit-coder")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("%s answered %d: %s", CatalogURL, resp.StatusCode, body)
	}
	return ParseCatalog(resp.Body, now)
}

// ParseCatalog reads models.dev's api.json, keeping only the providers in
// Providers. The other two hundred or so are decoded no further than their
// raw bytes.
func ParseCatalog(r io.Reader, now time.Time) (*Catalog, error) {
	var all map[string]json.RawMessage
	if err := json.NewDecoder(r).Decode(&all); err != nil {
		return nil, fmt.Errorf("reading the models.dev catalog: %w", err)
	}
	c := &Catalog{FetchedAt: now, Providers: map[string]CatalogProvider{}}
	for _, id := range CatalogIDs() {
		raw, ok := all[id]
		if !ok {
			continue
		}
		var p CatalogProvider
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("reading provider %s in the models.dev catalog: %w", id, err)
		}
		c.Providers[id] = p
	}
	if len(c.Providers) == 0 {
		return nil, errors.New("the models.dev catalog lists none of the providers kvit-coder knows")
	}
	return c, nil
}

func readCatalogCache(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &c, nil
}

func writeCatalogCache(path string, c *Catalog) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
