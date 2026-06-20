package benchmark

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// TBCSVHeaders are the columns for the thinkbench results CSV (used for resume).
var TBCSVHeaders = []string{
	"slug", "type", "run", "observed",
	"score", "passed", "total", "full_pass", "import_ok",
	"llm_calls", "tokens", "prompt_tokens", "generated_tokens", "cached_tokens",
	"context_used", "cost", "duration_ms", "errors", "workspace_dir", "failed_checks",
}

// TBCSVWriter appends thinkbench run results to a CSV for resume.
type TBCSVWriter struct {
	file   *os.File
	writer *csv.Writer
}

// NewTBCSVWriter opens (or creates) the CSV. When resume is true it appends.
func NewTBCSVWriter(path string, resume bool) (*TBCSVWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	var file *os.File
	var err error
	if resume {
		if _, statErr := os.Stat(path); statErr == nil {
			file, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
		} else {
			file, err = os.Create(path)
		}
	} else {
		file, err = os.Create(path)
	}
	if err != nil {
		return nil, err
	}

	w := &TBCSVWriter{file: file, writer: csv.NewWriter(file)}
	if info, _ := file.Stat(); info.Size() == 0 {
		if err := w.writer.Write(TBCSVHeaders); err != nil {
			return nil, err
		}
		w.writer.Flush()
	}
	return w, nil
}

// WriteResult appends one result row.
func (w *TBCSVWriter) WriteResult(r *TBRunResult) error {
	errorsJSON, _ := json.Marshal(r.Errors)
	failedJSON, _ := json.Marshal(r.FailedChecks)
	row := []string{
		r.Slug,
		r.Type,
		strconv.Itoa(r.Run),
		strconv.FormatBool(r.Observed),
		fmt.Sprintf("%.4f", r.Score),
		strconv.Itoa(r.Passed),
		strconv.Itoa(r.Total),
		strconv.FormatBool(r.FullPass),
		strconv.FormatBool(r.ImportOK),
		strconv.Itoa(r.LLMCalls),
		strconv.Itoa(r.Tokens),
		strconv.Itoa(r.PromptTokens),
		strconv.Itoa(r.GeneratedTokens),
		strconv.Itoa(r.CachedTokens),
		strconv.Itoa(r.ContextUsed),
		fmt.Sprintf("%.6f", r.Cost),
		strconv.FormatInt(r.DurationMS, 10),
		string(errorsJSON),
		r.WorkspaceDir,
		string(failedJSON),
	}
	if err := w.writer.Write(row); err != nil {
		return err
	}
	w.writer.Flush()
	return w.writer.Error()
}

// Close flushes and closes the CSV.
func (w *TBCSVWriter) Close() error {
	w.writer.Flush()
	return w.file.Close()
}

// LoadTBResults loads previously recorded thinkbench results from CSV.
func LoadTBResults(path string) ([]TBRunResult, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, nil
	}

	var out []TBRunResult
	for _, row := range records[1:] {
		if len(row) < 19 { // 19 = original column count; failed_checks (col 20) is optional
			continue
		}
		run, _ := strconv.Atoi(row[2])
		score, _ := strconv.ParseFloat(row[4], 64)
		passed, _ := strconv.Atoi(row[5])
		total, _ := strconv.Atoi(row[6])
		llmCalls, _ := strconv.Atoi(row[9])
		tokens, _ := strconv.Atoi(row[10])
		promptTokens, _ := strconv.Atoi(row[11])
		genTokens, _ := strconv.Atoi(row[12])
		cachedTokens, _ := strconv.Atoi(row[13])
		contextUsed, _ := strconv.Atoi(row[14])
		cost, _ := strconv.ParseFloat(row[15], 64)
		durationMS, _ := strconv.ParseInt(row[16], 10, 64)

		var errs []string
		if row[17] != "" && row[17] != "null" {
			_ = json.Unmarshal([]byte(row[17]), &errs)
		}

		var failed []string
		if len(row) >= 20 && row[19] != "" && row[19] != "null" {
			_ = json.Unmarshal([]byte(row[19]), &failed)
		}

		out = append(out, TBRunResult{
			Slug:            row[0],
			Type:            row[1],
			Run:             run,
			Observed:        strings.EqualFold(row[3], "true"),
			Score:           score,
			Passed:          passed,
			Total:           total,
			FullPass:        strings.EqualFold(row[7], "true"),
			ImportOK:        strings.EqualFold(row[8], "true"),
			LLMCalls:        llmCalls,
			Tokens:          tokens,
			PromptTokens:    promptTokens,
			GeneratedTokens: genTokens,
			CachedTokens:    cachedTokens,
			ContextUsed:     contextUsed,
			Cost:            cost,
			DurationMS:      durationMS,
			Errors:          errs,
			WorkspaceDir:    row[18],
			FailedChecks:    failed,
		})
	}
	return out, nil
}

// tbCompletedKey identifies a completed (slug, run) for resume.
func tbCompletedKey(slug string, run int) string {
	return fmt.Sprintf("%s#%d", slug, run)
}
