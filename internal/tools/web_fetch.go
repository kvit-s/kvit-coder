package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
)

const (
	defaultFetchTimeout   = 30 * time.Second
	defaultFetchMaxBytes  = int64(5 << 20) // 5 MB of HTML before we stop reading
	defaultFetchUserAgent = "kvit-coder (+https://github.com/kvit-s/kvit-coder)"

	// A page whose conversion is this short and whose source was this long was
	// almost certainly rendered by JavaScript that never ran, or answered with
	// a bot check. Both conditions are needed: example.com converts to 167
	// bytes from 559 of HTML and is a real page, whereas an application shell
	// is kilobytes of script around an empty div. Saying so is what tells the
	// model to reach for a browser rather than concluding the page was empty.
	emptyPageThreshold = 200
	jsShellMinHTML     = 2048
)

// WebFetchTool retrieves one URL and converts it to markdown.
type WebFetchTool struct {
	config      *config.Config
	tempFileMgr *TempFileManager
	client      *http.Client
}

func NewWebFetchTool(cfg *config.Config, tempFileMgr *TempFileManager) *WebFetchTool {
	return &WebFetchTool{
		config:      cfg,
		tempFileMgr: tempFileMgr,
		client: &http.Client{
			Timeout: fetchTimeout(cfg),
			// A cross-host redirect is followed, but the URL that was actually
			// read comes back as final_url so the model is never wrong about
			// which page it is looking at.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				return nil
			},
		},
	}
}

func fetchTimeout(cfg *config.Config) time.Duration {
	if s := cfg.Tools.Web.Fetch.Timeout; s > 0 {
		return time.Duration(s) * time.Second
	}
	return defaultFetchTimeout
}

func (t *WebFetchTool) Name() string { return "Web.fetch" }

func (t *WebFetchTool) Description() string {
	return "Fetch a web page and return it as markdown. Retrieves static HTML only: a page whose content is rendered by JavaScript comes back empty and says so."
}

// ParallelSafe says several fetches can run at once in a Batch: the tool only
// reads, and the network is the thing being waited on.
func (t *WebFetchTool) ParallelSafe() bool { return true }

// SelfTimeout opts out of the loop's blanket 15-second tool timeout. A page can
// be slow in a way an API is not, and the client applies its own deadline.
func (t *WebFetchTool) SelfTimeout() bool { return true }

func (t *WebFetchTool) Check(ctx context.Context, args json.RawMessage) error { return nil }

func (t *WebFetchTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "Absolute http:// or https:// URL to fetch",
			},
		},
		"required": []string{"url"},
	}
}

func (t *WebFetchTool) PromptCategory() string     { return "web" }
func (t *WebFetchTool) PromptOrder() int           { return 20 } // after Web.search
func (t *WebFetchTool) PromptTemplateName() string { return "" }

func (t *WebFetchTool) PromptSection() string {
	return `### Web.fetch - Read a Web Page

**Usage:** ` + "`" + `Web.fetch {"url": "https://..."}` + "`" + `

The page is converted to markdown and written to a file. A short page comes back
whole in ` + "`content`" + `; a long one comes back as ` + "`head`" + ` plus an
` + "`outline`" + ` of its headings with line numbers, and you read the parts you
want with ` + "`" + `Read {"path": ..., "offset": ..., "limit": ...}` + "`" + `.

Only static HTML is retrieved. A page that builds its content with JavaScript,
or one that answers with a bot check, comes back nearly empty and the result
says so rather than pretending the page had nothing on it.

Prefer a plain-text address when the site publishes one: ` + "`raw.githubusercontent.com`" + `
over the GitHub file view, ` + "`pkg.go.dev`" + ` over a rendered documentation
site, an ` + "`llms.txt`" + ` or ` + "`.txt`" + ` variant where it exists. Those
need no rendering and no browser.`
}

func (t *WebFetchTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

	target, err := validateFetchURL(params.URL)
	if err != nil {
		return nil, err
	}

	// The ceiling on how much source is downloaded is a safety limit from the
	// config, not something the caller chooses. It used to be a parameter, and
	// a model that read it as a cap on the *result* set it to 20 KB, truncating
	// a 50 KB document mid-tree into a fragment that converted to almost
	// nothing. The size control the caller actually wants is the outline and a
	// Read of the lines it names.
	maxBytes := t.config.Tools.Web.Fetch.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultFetchMaxBytes
	}

	// A page fetched twice in one session is read from disk the second time.
	if cached, ok := t.cached(target.String()); ok {
		return t.present(target.String(), target.String(), 200, cached.title, cached.markdown, cached.path, len(cached.markdown), true), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", t.userAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.9,*/*;q=0.8")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", target, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", target, err)
	}

	finalURL := target.String()
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}

	if resp.StatusCode >= 400 {
		return map[string]any{
			"url":       target.String(),
			"final_url": finalURL,
			"status":    resp.StatusCode,
			"error":     fmt.Sprintf("the server answered %s", resp.Status),
		}, nil
	}

	sourceTruncated := int64(len(raw)) >= maxBytes

	markdown, title, err := t.convert(resp.Header.Get("Content-Type"), raw)
	if err != nil {
		return nil, err
	}

	var path string
	// A page that needed a browser is not cached: it holds no content worth
	// keeping, and the site may well work later.
	if !looksLikeJSShell(markdown, len(raw)) {
		if p, err := t.spill(target.String(), markdown); err == nil {
			path = p
		}
		// Losing the file is not worth losing the page: what fits is still
		// returned, just without somewhere to read the rest from.
	}
	out := t.present(target.String(), finalURL, resp.StatusCode, title, markdown, path, len(raw), false)
	if sourceTruncated {
		out["source_truncated"] = true
		out["warning"] = fmt.Sprintf(
			"the page is larger than the %d-byte download ceiling, so its HTML was cut off part-way and what follows is incomplete. Raise tools.web.fetch.max_bytes if this page matters.", maxBytes)
	}
	return out, nil
}

// convert turns the response body into markdown. Content that is already text
// is passed through, since running a plain-text file through an HTML parser
// only risks mangling it.
func (t *WebFetchTool) convert(contentType string, raw []byte) (string, string, error) {
	ct := strings.ToLower(contentType)
	isHTML := strings.Contains(ct, "html") || strings.Contains(ct, "xml")
	if ct != "" && !isHTML {
		return strings.TrimSpace(string(raw)), "", nil
	}
	return convertPage(raw)
}

func (t *WebFetchTool) userAgent() string {
	if ua := strings.TrimSpace(t.config.Tools.Web.Fetch.UserAgent); ua != "" {
		return ua
	}
	return defaultFetchUserAgent
}

// present builds the tool result. Which shape it takes is decided by size,
// against the same caps the Read tool uses: a page that fits comes back whole,
// and one that does not comes back as an outline saying where to look. Making
// the model take two steps to read three kilobytes is the cost this avoids.
// looksLikeJSShell reports the one failure that otherwise looks like success:
// a fetch that returned 200 and almost no text because the content was never
// rendered.
func looksLikeJSShell(markdown string, rawBytes int) bool {
	return len(strings.TrimSpace(markdown)) < emptyPageThreshold && rawBytes >= jsShellMinHTML
}

func (t *WebFetchTool) present(requested, final string, status int, title, markdown, path string, rawBytes int, fromCache bool) map[string]any {
	lines := strings.Split(markdown, "\n")

	result := map[string]any{
		"url":    requested,
		"status": status,
		"lines":  len(lines),
		"bytes":  len(markdown),
	}
	if final != "" && final != requested {
		result["final_url"] = final
		result["note"] = "redirected: this is the content of final_url, not of url"
	}
	if title != "" {
		result["title"] = title
	}
	if path != "" {
		result["path"] = path
	}
	if fromCache {
		result["cached"] = true
	}

	if looksLikeJSShell(markdown, rawBytes) {
		result["content"] = markdown
		result["warning"] = "almost nothing was extracted from a page of this size, so it probably renders its content with JavaScript or answered with a bot check. Static fetching cannot read it; a browser can."
		return result
	}

	maxLines := t.config.Tools.Read.MaxPartialLines
	if maxLines <= 0 {
		maxLines = 150
	}
	maxBytes := t.config.Tools.Read.MaxReadSizeKB * 1024
	if maxBytes <= 0 {
		maxBytes = 24 * 1024
	}

	if len(lines) <= maxLines && len(markdown) <= maxBytes {
		result["content"] = markdown
		return result
	}

	// Too large: an outline plus the opening lines. Truncating at the top alone
	// would give the introduction and then stop, which is the least useful
	// slice of a long page.
	headLines := maxLines / 4
	if headLines < 10 {
		headLines = 10
	}
	if headLines > len(lines) {
		headLines = len(lines)
	}
	result["head"] = strings.Join(lines[:headLines], "\n")
	result["outline"] = buildOutline(markdown)
	result["truncated"] = true
	if path != "" && t.config.Tools.Read.Enabled {
		result["hint"] = fmt.Sprintf(
			"The whole page is at that path. Read a section with Read {\"path\": %q, \"offset\": <line from outline>, \"limit\": %d}",
			path, maxLines)
	}
	return result
}

type cachedPage struct {
	markdown string
	title    string
	path     string
}

// cachePath names a fetch by its URL, so the same page fetched twice in one
// session is read from disk the second time.
func (t *WebFetchTool) cachePath(rawURL string) string {
	if t.tempFileMgr == nil {
		return ""
	}
	dir := t.tempFileMgr.TempDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(rawURL))
	return filepath.Join(dir, "fetch-"+hex.EncodeToString(sum[:])[:12]+".md")
}

func (t *WebFetchTool) cached(rawURL string) (cachedPage, bool) {
	path := t.cachePath(rawURL)
	if path == "" {
		return cachedPage{}, false
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return cachedPage{}, false
	}
	md := string(body)
	title := ""
	if line, _, ok := strings.Cut(md, "\n"); ok && strings.HasPrefix(line, "# ") {
		title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
	}
	return cachedPage{markdown: md, title: title, path: path}, true
}

func (t *WebFetchTool) spill(rawURL, markdown string) (string, error) {
	path := t.cachePath(rawURL)
	if path == "" {
		return "", fmt.Errorf("no temp file manager")
	}
	if err := os.WriteFile(path, []byte(markdown), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// validateFetchURL refuses anything that is not an absolute http(s) URL, which
// keeps file:// and the other schemes the http client would otherwise be asked
// about out of a tool whose whole job is reaching the network.
func validateFetchURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("not a valid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("only http and https urls can be fetched, got %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("url has no host: %s", raw)
	}
	return u, nil
}
