package benchmark

// This file defines the data model for the thinkbench benchmark family — a
// parallel family to haystack that runs autonomous coding-agent tasks graded by
// a held-out python grader (see thinkbench-plan.md / thinkbench.md). It reads
// the suite directly from disk (no YAML inlining) and produces continuous
// passed/total scores rather than the binary pass/fail of the tool benchmarks.

// TBType is a thinkbench task type.
type TBType string

const (
	TBImplement     TBType = "implement"
	TBBugFix        TBType = "bug-fix"
	TBFeatureAdd    TBType = "feature-add"
	TBRepairToGreen TBType = "repair-to-green"
	TBAmbiguousSpec TBType = "ambiguous-spec" // observed, ungraded
)

// TBTask is a single thinkbench task loaded from the suite directory.
type TBTask struct {
	Slug       string
	Type       TBType
	Brief      string // brief.txt contents (the only prompt the agent sees)
	Observed   bool   // ambiguous-spec → true (no grader, persist-only)
	NumChecks  int    // from manifest (0 for observed)
	HasSetup   bool   // has a setup/ starter tree
	SetupDir   string // abs path to suite/tasks/<slug>/setup (may not exist)
	GraderPath string // abs path to suite/tasks/<slug>/grade.py (held out; "" for observed)
}

// TBManifestEntry mirrors one entry in the suite manifest.json.
type TBManifestEntry struct {
	Slug      string `json:"slug"`
	Type      string `json:"type"`
	Brief     string `json:"brief"`
	Observed  bool   `json:"observed"`
	NumChecks int    `json:"num_checks"`
	HasSetup  bool   `json:"has_setup"`
}

// TBCheck is one behavioral check within a grader scorecard.
type TBCheck struct {
	ID     string `json:"id"`
	Desc   string `json:"desc"`
	Detail string `json:"detail"`
	Passed bool   `json:"passed"`
}

// TBScorecard is the JSON scorecard a grade.py prints to stdout.
type TBScorecard struct {
	Task     string    `json:"task"`
	ImportOK bool      `json:"import_ok"`
	Passed   int       `json:"passed"`
	Total    int       `json:"total"`
	Score    float64   `json:"score"`
	Checks   []TBCheck `json:"checks"`
}

// TBRunResult captures the outcome and metrics of one (task, trial) run.
type TBRunResult struct {
	Slug     string
	Type     string
	Run      int
	Observed bool

	// Scoring (graded tasks only; observed tasks leave these zero).
	Score    float64 // 0..1 = passed/total
	Passed   int
	Total    int
	FullPass bool // Score == 1.0 — the binary "solve" analogue
	ImportOK bool

	// Metrics (reused from agent stats, mirroring RunResult).
	DurationMS      int64
	Tokens          int
	PromptTokens    int
	GeneratedTokens int
	CachedTokens    int
	ContextUsed     int
	Cost            float64
	LLMCalls        int

	Errors       []string
	WorkspaceDir string // persisted path for observed tasks / grading failures
}

// CachedFrac returns the cached fraction of prompt tokens for this run.
func (r *TBRunResult) CachedFrac() float64 {
	if r.PromptTokens <= 0 {
		return 0
	}
	return float64(r.CachedTokens) / float64(r.PromptTokens)
}
