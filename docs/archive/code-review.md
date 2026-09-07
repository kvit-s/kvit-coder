# Deep Code Review: kvit-coder

## Executive Summary

kvit-coder is a well-architected minimal coding agent written in Go. The codebase demonstrates solid engineering practices with a clean separation of concerns, comprehensive error handling, and thoughtful security considerations. However, there are areas for improvement in test coverage, error handling consistency, and potential security hardening.

---

## Architecture Overview

**Strengths:**
- Clean two-binary architecture (headless `kvit-coder` + interactive `kvit-coder-ui`)
- Well-designed plugin system for tools via the `Tool` interface
- Proper use of context for cancellation propagation
- Configurable LLM backend (OpenAI-compatible API)
- Session persistence with JSONL format
- Built-in benchmarking system for testing LLM tool usage

---

## Code Quality Analysis

### Strengths

1. **Error Classification System** (`internal/tools/errors.go`)
   - Distinguishes between `SemanticError` (backtrackable, LLM misuse) and `RuntimeError` (not backtrackable)
   - Enables intelligent retry behavior

2. **Loop Detection** (`internal/agent/loopdetector.go`)
   - Detects repeated identical calls
   - Alternating loop detection (A->B->A->B pattern)
   - Error loop detection (same tool failing repeatedly)

3. **Backtrack System** (`internal/agent/backtrack.go`)
   - Smart retry with token/cost tracking for discarded attempts
   - Configurable max retries with user message injection on limit

4. **Streaming File Reading** (`internal/tools/filesystem.go:513-636`)
   - Memory-efficient circular buffer for reading last N lines
   - Byte position tracking for efficient seeking

### Issues & Recommendations

1. **Test Coverage is Low**
   ```
   internal/agent         0.0%
   internal/benchmark     0.0%
   internal/tools        34.9%
   internal/config       28.3%
   internal/llm          41.0%
   ```
   The agent core logic has no tests. This is risky for the most critical component.

2. **Global State** (`internal/tools/filesystem.go:33, 107, 121`)
   ```go
   var globalReadTracker = &FileReadTracker{maxEntries: 10}
   var globalPendingEdit *pendingEdit
   var globalPendingWrite *pendingWrite
   ```
   Global mutable state makes testing difficult and can cause issues in concurrent scenarios. Consider dependency injection.

3. **Large Functions** (`internal/agent/runner.go:86-994`)
   The `Run` method is ~900 lines with deeply nested control flow. Should be refactored into smaller, testable units.

4. **Magic Numbers** (`internal/agent/runner.go`)
   ```go
   const maxConsecutiveDuplicates = 3
   const maxProviderFailures = 2
   const maxContextOverflowRetries = 2
   ```
   These should be configurable or at least documented with rationale.

5. **Error Handling Inconsistency**
   Some functions return `(any, error)` and return `nil` for errors in the map:
   ```go
   return map[string]any{
       "success": false,
       "error":   "file_not_found",
   }, nil  // Returns nil error but failure in map
   ```
   This pattern is confusing. Consider consistent error returns.

6. **Potential Memory Leak** (`internal/tools/filesystem.go:51-57`)
   ```go
   t.readFiles = append(t.readFiles, ...)
   if len(t.readFiles) > t.maxEntries*5 {
       t.readFiles = t.readFiles[len(t.readFiles)-t.maxEntries*5:]
   }
   ```
   Growing to 5x entries before trimming wastes memory. Consider ring buffer.

7. **Duplicate Code**
   Session management code is duplicated between `cmd/kvit-coder/main.go:70-114` and `cmd/kvit-coder-ui/main.go:43-88`. Extract to shared function.

---

## Specific File Issues

### `internal/llm/client.go`

1. **Line 24**: `DisableKeepAlives: true` - This may hurt performance. Consider connection pooling for production.

2. **Line 170-173**: Workaround for llama.cpp bug:
   ```go
   if parseErr != nil && readErr != nil {
       fixedBody := append(respBody, '}')
   ```
   This is fragile. Better to detect and log the issue rather than silently fix.

### `internal/tools/shell.go`

1. **Line 208-216**: Timeout handling creates response but doesn't return immediately - execution continues. This is correct but could be clearer.

2. **Line 443**: `pathPattern` regex may not correctly extract all paths:
   ```go
   pathPattern := regexp.MustCompile(`(?:^|\s)([~/.][\w\-./~]+|/[\w\-./~]+)`)
   ```
   Doesn't handle quoted paths with spaces.

### `internal/agent/runner.go`

1. **Lines 170-172**: Closure to capture `iterCancel` is overly complex:
   ```go
   cancelOnce := func(c context.CancelFunc) func() {
       return func() { c() }
   }(iterCancel)
   ```
   Simpler: just use `defer iterCancel()`.

2. **Line 208**: 10ms sleep after closing `llmDone` channel:
   ```go
   time.Sleep(10 * time.Millisecond)
   ```
   Workaround for a race condition. Should use proper synchronization.

---

## Recommendations

### High Priority

1. **Add Tests for Agent Core** - The `internal/agent` package has 0% coverage despite being the most critical component.

2. **Refactor Global State** - Replace `globalPendingEdit`, `globalReadTracker` with injected dependencies.

3. **Harden Shell Command Validation** - Expand blocklist, consider stricter parsing.

4. **Break Up Large Functions** - `runner.go:Run()` should be split into smaller methods.

### Medium Priority

5. **Add Integration Tests** - Test end-to-end agent flows with mock LLM.

6. **Use `crypto/rand`** - Replace `math/rand` in session name generation.

7. **Configuration Documentation** - Add `config.example.yaml` with all options documented.

8. **Logging Improvements** - Structured logging with levels, currently using simple zap.

### Low Priority

9. **Connection Pooling** - Re-enable `KeepAlives` for better performance.

10. **Memory Optimization** - Use ring buffer for read tracker instead of slice trimming.

11. **Code Deduplication** - Extract common session management code.

---

## Overall Assessment

**Rating: 7.5/10**

The codebase is well-structured for its purpose with thoughtful security measures. Main concerns are:
- Low test coverage in critical areas
- Large monolithic functions that need refactoring
- Some security hardening opportunities in shell execution

The architecture is sound and extensible. With improved testing and the recommended security enhancements, this would be a robust coding agent implementation.
