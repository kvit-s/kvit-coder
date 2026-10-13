package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// PlanWindow is one limit of a subscription plan as the provider last
// reported it. The JSON names are the ones kvit-works keeps Claude Code's
// plan limits under (its core/planusage.Window), so a client that shows both
// reads them the same way.
type PlanWindow struct {
	// Name is the window: "five_hour", "seven_day" or "monthly".
	Name string `json:"window"`
	// Used is the share of the limit used, from 0 to 1, nil when the
	// provider did not say.
	Used *float64 `json:"utilization,omitempty"`
	// ResetsAt is when the window starts again, zero when not said.
	ResetsAt time.Time `json:"resetsAt,omitzero"`
	// Status is "allowed", or "rejected" once the limit is reached; "" when
	// the provider said something else.
	Status string `json:"status,omitempty"`
}

// planUsageTimeout bounds the one request PlanUsage makes. The reading is
// optional and the end of a turn waits for it, so a slow answer is dropped.
const planUsageTimeout = 5 * time.Second

// planUsageURL is where the endpoint at baseURL says how much of its plan's
// limits have been used, "" for an endpoint with no such place.
//
// Only OpenCode Go has one: GET <base>/usage at https://opencode.ai/zen/go/v1
// (or /inference/go/v1), with the same key as the model requests. It answers
//
//	{"usage":{"rolling":{"status":"ok","percent":37,"resetsAt":"2026-10-13T08:08:22.000Z"},
//	          "weekly":{…},"monthly":{…}}}
//
// It is OpenCode's own (its console's gateway added it in August 2026) and
// undocumented; nothing on a model response says the same, so this is the only
// way to learn the use before a request is refused.
func planUsageURL(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil || !strings.EqualFold(u.Hostname(), "opencode.ai") {
		return ""
	}
	if !strings.HasSuffix(strings.TrimSuffix(u.Path, "/"), "/go/v1") {
		return ""
	}
	return strings.TrimSuffix(baseURL, "/") + "/usage"
}

// openCodeWindows names OpenCode Go's windows the way PlanWindow does. Its
// "rolling" window is five hours long, starting at the first request after
// the last one ended; "weekly" runs from Monday 00:00 UTC.
var openCodeWindows = map[string]string{
	"rolling": "five_hour",
	"weekly":  "seven_day",
	"monthly": "monthly",
}

// planWindowOrder is the order PlanUsage returns windows in, any window not
// listed after these by name.
var planWindowOrder = []string{"five_hour", "seven_day", "monthly"}

// PlanUsage asks the endpoint how much of its subscription plan's limits
// have been used. It returns nil and no error for an endpoint with no plan to
// ask about. It makes one request, without the retry ladder Chat has.
func (c *Client) PlanUsage(ctx context.Context) ([]PlanWindow, error) {
	if c.planUsageURL == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, planUsageTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.planUsageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	// The session and User-Agent headers go too: OpenCode has flagged usage
	// requests without them as coming from an unknown client.
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}
	if c.auth != nil {
		if err := c.auth.Authorize(ctx, httpReq); err != nil {
			return nil, err
		}
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.statusError(resp.StatusCode, respBody)
	}
	return parseOpenCodeUsage(respBody)
}

// parseOpenCodeUsage reads OpenCode Go's usage answer (planUsageURL).
func parseOpenCodeUsage(body []byte) ([]PlanWindow, error) {
	var answer struct {
		Usage map[string]struct {
			Status   string   `json:"status"`
			Percent  *float64 `json:"percent"`
			ResetsAt string   `json:"resetsAt"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, fmt.Errorf("decode response: %w (body preview: %s)", err, bodyPreview(body))
	}
	var out []PlanWindow
	for name, window := range answer.Usage {
		w := PlanWindow{Name: name}
		if mapped, ok := openCodeWindows[name]; ok {
			w.Name = mapped
		}
		if window.Percent != nil {
			used := max(0, *window.Percent) / 100
			w.Used = &used
		}
		if at, err := time.Parse(time.RFC3339, window.ResetsAt); err == nil {
			w.ResetsAt = at.UTC()
		}
		switch window.Status {
		case "ok":
			w.Status = "allowed"
		case "rate-limited":
			w.Status = "rejected"
		}
		out = append(out, w)
	}
	rank := func(name string) int {
		if i := slices.Index(planWindowOrder, name); i >= 0 {
			return i
		}
		return len(planWindowOrder)
	}
	slices.SortFunc(out, func(a, b PlanWindow) int {
		if ra, rb := rank(a.Name), rank(b.Name); ra != rb {
			return ra - rb
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out, nil
}
