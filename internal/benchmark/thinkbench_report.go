package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// tbRollup is the per-model aggregate emitted for each rollup bucket, matching
// the example thinkbench results.json schema.
type tbRollup struct {
	N            int     `json:"n"`
	Solved       int     `json:"solved"`
	SolveRate    float64 `json:"solve_rate"`
	MeanScore    float64 `json:"mean_score"`
	AvgSecs      float64 `json:"avg_secs"`
	AvgTokens    int     `json:"avg_tokens"`
	CachedFrac   float64 `json:"cached_frac"`
	TotalCostUSD float64 `json:"total_cost_usd"`
}

// tbByTaskEntry is the per-task block in results.json.
type tbByTaskEntry struct {
	Type   string              `json:"type"`
	Models map[string]tbRollup `json:"models"`
}

// TBResults is the top-level results.json document.
type TBResults struct {
	RunIDs           []int64                        `json:"run_ids"`
	Models           []string                       `json:"models"`
	Trials           int                            `json:"trials"`
	NumGradedTasks   int                            `json:"num_graded_tasks"`
	NumObservedTasks int                            `json:"num_observed_tasks"`
	NumRuns          int                            `json:"num_runs"`
	EffortMode       string                         `json:"effort_mode"`
	Config           map[string]any                 `json:"config"`
	OverallGraded    map[string]tbRollup            `json:"overall_graded"`
	ByType           map[string]map[string]tbRollup `json:"by_type"`
	ByTask           map[string]tbByTaskEntry       `json:"by_task"`
	Observed         map[string]map[string]tbRollup `json:"observed"`
}

// rollup aggregates a set of run results into a tbRollup.
func rollup(results []TBRunResult) tbRollup {
	n := len(results)
	if n == 0 {
		return tbRollup{}
	}
	var solved int
	var sumScore, sumSecs, sumCost float64
	var sumTokens, sumPrompt, sumCached int
	for _, r := range results {
		if r.FullPass {
			solved++
		}
		sumScore += r.Score
		sumSecs += float64(r.DurationMS) / 1000.0
		sumCost += r.Cost
		sumTokens += r.Tokens
		sumPrompt += r.PromptTokens
		sumCached += r.CachedTokens
	}
	cachedFrac := 0.0
	if sumPrompt > 0 {
		cachedFrac = float64(sumCached) / float64(sumPrompt)
	}
	return tbRollup{
		N:            n,
		Solved:       solved,
		SolveRate:    round4(float64(solved) / float64(n)),
		MeanScore:    round4(sumScore / float64(n)),
		AvgSecs:      round1(sumSecs / float64(n)),
		AvgTokens:    int(float64(sumTokens)/float64(n) + 0.5),
		CachedFrac:   round4(cachedFrac),
		TotalCostUSD: round4(sumCost),
	}
}

// BuildTBResults assembles the results document from all run results.
func BuildTBResults(model string, runID int64, trials int, tasks []TBTask, results []TBRunResult, config map[string]any) *TBResults {
	graded := filterResults(results, false)
	observed := filterResults(results, true)

	// Task → type lookup.
	taskType := make(map[string]string, len(tasks))
	for _, t := range tasks {
		taskType[t.Slug] = string(t.Type)
	}

	res := &TBResults{
		RunIDs:           []int64{runID},
		Models:           []string{model},
		Trials:           trials,
		NumGradedTasks:   countTasks(tasks, false),
		NumObservedTasks: countTasks(tasks, true),
		NumRuns:          len(results),
		EffortMode:       "none",
		Config:           config,
		OverallGraded:    map[string]tbRollup{model: rollup(graded)},
		ByType:           map[string]map[string]tbRollup{},
		ByTask:           map[string]tbByTaskEntry{},
		Observed:         map[string]map[string]tbRollup{},
	}

	// by_type (graded only).
	byType := groupBy(graded, func(r TBRunResult) string { return r.Type })
	for typ, rs := range byType {
		res.ByType[typ] = map[string]tbRollup{model: rollup(rs)}
	}

	// by_task (graded only).
	byTask := groupBy(graded, func(r TBRunResult) string { return r.Slug })
	for slug, rs := range byTask {
		res.ByTask[slug] = tbByTaskEntry{
			Type:   taskType[slug],
			Models: map[string]tbRollup{model: rollup(rs)},
		}
	}

	// observed.
	byObserved := groupBy(observed, func(r TBRunResult) string { return r.Slug })
	for slug, rs := range byObserved {
		res.Observed[slug] = map[string]tbRollup{model: rollup(rs)}
	}

	return res
}

// WriteTBResultsJSON writes results.json.
func WriteTBResultsJSON(path string, res *TBResults) error {
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// WriteTBResultsMarkdown writes the human-readable RESULTS.md, mirroring the
// example bundle's headings (overall, by-type, per-task, observed).
func WriteTBResultsMarkdown(path string, model string, res *TBResults) error {
	var b strings.Builder

	b.WriteString("# thinkbench — results\n\n")
	fmt.Fprintf(&b, "%d runs · %d graded tasks + %d observed · %d trials · model: %s · thinking mode: %s\n\n",
		res.NumRuns, res.NumGradedTasks, res.NumObservedTasks, res.Trials, model, res.EffortMode)

	// Configuration.
	b.WriteString("## Configuration\n\n")
	if prov, ok := res.Config["provider"].(string); ok {
		fmt.Fprintf(&b, "Provider: **%v**", prov)
		if ep, ok := res.Config["endpoint"].(string); ok {
			fmt.Fprintf(&b, " · endpoint `%v`", ep)
		}
		fmt.Fprintf(&b, " · model `%s` · %d trials/task · cache-aware cost.\n\n", model, res.Trials)
	}

	// Overall.
	b.WriteString("## Overall (graded tasks)\n\n")
	b.WriteString("| model | full-pass | mean score | avg latency | avg tokens | cached | total cost |\n")
	b.WriteString("|---|--:|--:|--:|--:|--:|--:|\n")
	ov := res.OverallGraded[model]
	fmt.Fprintf(&b, "| %s | %d/%d (%.0f%%) | %.3f | %.0fs | %s | %.0f%% | $%.2f |\n",
		model, ov.Solved, ov.N, ov.SolveRate*100, ov.MeanScore, ov.AvgSecs,
		commaInt(ov.AvgTokens), ov.CachedFrac*100, ov.TotalCostUSD)
	b.WriteString("\n")

	// By type.
	b.WriteString("## By task type (mean score / full-pass rate)\n\n")
	b.WriteString("| type | tasks | mean score | full-pass |\n")
	b.WriteString("|---|--:|--:|--:|\n")
	for _, typ := range []string{"implement", "bug-fix", "feature-add", "repair-to-green"} {
		m, ok := res.ByType[typ]
		if !ok {
			continue
		}
		r := m[model]
		fmt.Fprintf(&b, "| %s | %d | %.3f | %.0f%% |\n", typ, r.N, r.MeanScore, r.SolveRate*100)
	}
	b.WriteString("\n")

	// Per-task.
	b.WriteString("## Per-task (graded — mean score over trials)\n\n")
	b.WriteString("| task | type | mean score | full-pass |\n")
	b.WriteString("|---|---|--:|--:|\n")
	slugs := sortedKeys(res.ByTask)
	for _, slug := range slugs {
		entry := res.ByTask[slug]
		r := entry.Models[model]
		fmt.Fprintf(&b, "| %s | %s | %.2f | %d/%d |\n", slug, entry.Type, r.MeanScore, r.Solved, r.N)
	}
	b.WriteString("\n")

	// Observed.
	if len(res.Observed) > 0 {
		b.WriteString("## Ambiguous-spec (observed — NOT scored)\n\n")
		b.WriteString("_These probe how the model interprets an underspecified brief; there is no right answer, so no score. The solution + transcript are persisted per run for the qualitative read. Latency / tokens / cost below are descriptive only._\n\n")
		b.WriteString("| task | lat / tok / $ |\n")
		b.WriteString("|---|--:|\n")
		for _, slug := range sortedKeysObserved(res.Observed) {
			r := res.Observed[slug][model]
			fmt.Fprintf(&b, "| %s | %.0fs / %s / $%.3f |\n", slug, r.AvgSecs, commaInt(r.AvgTokens), r.TotalCostUSD)
		}
		b.WriteString("\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0644)
}

// --- helpers ---------------------------------------------------------------

func filterResults(results []TBRunResult, observed bool) []TBRunResult {
	var out []TBRunResult
	for _, r := range results {
		if r.Observed == observed {
			out = append(out, r)
		}
	}
	return out
}

func countTasks(tasks []TBTask, observed bool) int {
	n := 0
	for _, t := range tasks {
		if t.Observed == observed {
			n++
		}
	}
	return n
}

func groupBy(results []TBRunResult, key func(TBRunResult) string) map[string][]TBRunResult {
	m := make(map[string][]TBRunResult)
	for _, r := range results {
		k := key(r)
		m[k] = append(m[k], r)
	}
	return m
}

func sortedKeys(m map[string]tbByTaskEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysObserved(m map[string]map[string]tbRollup) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
func round4(f float64) float64 { return float64(int(f*10000+0.5)) / 10000 }

func commaInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 1000 {
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ",")
}
