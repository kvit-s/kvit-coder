# Safety Mode Enhancement Plan

Based on analysis of [claude-code-safety-net](https://github.com/kenryu42/claude-code-safety-net) and current kvit-coder safety implementation.

## Executive Summary

The current safety mode provides solid foundation with path permissions, dangerous command blocking, and user prompting. The claude-code-safety-net project offers several advanced features that would significantly enhance protection against destructive operations.

---

## Current State

### Existing Safety Features

| Feature | Location | Status |
|---------|----------|--------|
| Path safety modes (block/warn/ask_once/ask_always) | `config.go` | Implemented |
| Three-tier path permissions (denied/allowed/workspace) | `permissions.go` | Implemented |
| Path normalization & escape detection | `path_utils.go` | Implemented |
| Dangerous command blocklist | `shell.go` | Implemented |
| Pending edit blocking | `filesystem.go` | Implemented |
| User prompting for outside paths | `config.go`, `permissions.go` | Implemented |

### Current Command Blocking (shell.go:317-424)

- Privilege escalation: `sudo`, `su`, `chroot`
- Destructive: `rm -rf /`, `rm -rf ~`, `mkfs`, `dd if=`
- Network: `curl`, `wget`, `nc`, `netcat`
- Code execution: `python -c`, `perl -e`, `ruby -e`, `node -e`
- Package managers: `apt`, `yum`, `brew`

---

## Gap Analysis

| Feature | Current | Safety-Net | Gap |
|---------|---------|------------|-----|
| Semantic command parsing | Prefix matching | Full argument analysis | Missing |
| Git destructive ops | Not covered | Comprehensive | Missing |
| Shell wrapper detection | None | Recursive parsing | Missing |
| `find -delete` patterns | Not covered | Blocked | Missing |
| `xargs`/`parallel` chains | Not covered | Analyzed | Missing |
| Context-aware rm -rf | All blocked | Allow /tmp, cwd | Missing |
| Strict mode (fail-closed) | None | Optional | Missing |
| Paranoid mode | None | Optional | Missing |
| Audit logging | None | JSONL with redaction | Missing |
| Secret redaction | None | Automatic | Missing |

---

## Enhancement Proposals

### 1. Semantic Command Parser

**Priority:** High
**Location:** New `internal/tools/shell_parser.go`

Replace simple prefix matching with semantic argument parsing that understands:
- Flag combinations (`-rf` vs `-r -f` vs `--recursive --force`)
- Argument positions and values
- Quoted strings and escaping
- Variable expansion patterns

```go
type ParsedCommand struct {
    Binary    string
    Flags     map[string]bool
    FlagArgs  map[string]string  // e.g., -o filename
    Args      []string
    RawArgs   []string
}

func ParseCommand(cmd string) (*ParsedCommand, error)
```

**Benefits:**
- Detect `rm -r -f` same as `rm -rf`
- Understand `git push -f` same as `git push --force`
- Properly handle quoted arguments

---

### 2. Git Destructive Operation Blocking

**Priority:** High
**Location:** New `internal/tools/rules_git.go`

Block dangerous git operations that can cause data loss:

| Command | Risk | Action |
|---------|------|--------|
| `git push` (any variant) | User responsibility | Block |
| `git checkout -- <file>` | Discards uncommitted changes | Block |
| `git reset --hard` | Destroys uncommitted work | Block |
| `git stash drop` | Permanent stash deletion | Block |
| `git stash clear` | Deletes all stashes | Block |
| `git clean -f` | Removes untracked files | Block |
| `git clean --dry-run` / `-n` | Safe preview | Allow |
| `git branch -D` | Force delete branch | Warn |

**Implementation:**

```go
type GitRule struct {
    Pattern     string
    DangerFlags []string
    SafeFlags   []string
    Action      RuleAction // Block, Warn, Allow
    Message     string
}

var gitRules = []GitRule{
    {
        Pattern:     "git push",
        DangerFlags: []string{},  // Block all variants
        SafeFlags:   []string{},
        Action:      Block,
        Message:     "git push should be done by user directly",
    },
    {
        Pattern:     "git reset",
        DangerFlags: []string{"--hard"},
        Action:      Block,
        Message:     "git reset --hard destroys uncommitted work",
    },
    // ... more rules
}
```

---

### 3. Shell Wrapper Detection

**Priority:** High
**Location:** `internal/tools/shell_parser.go`

Recursively analyze commands wrapped in shell invocations:

```
bash -c 'rm -rf /'
sh -lc 'git reset --hard'
/bin/bash -c "dangerous command"
env bash -c 'command'
```

**Detection patterns:**
- `bash -c`, `sh -c`, `zsh -c`
- `env <shell> -c`
- `/bin/bash`, `/usr/bin/bash`, etc.
- Nested wrappers: `bash -c 'sh -c "cmd"'`

```go
func UnwrapShellCommand(cmd string) ([]string, error) {
    // Returns list of actual commands after unwrapping
    // Recursively handles nested wrappers
}
```

---

### 4. Find-Delete Pattern Blocking

**Priority:** Medium
**Location:** New `internal/tools/rules_find.go`

Block dangerous `find` patterns:

| Pattern | Action |
|---------|--------|
| `find ... -delete` | Block |
| `find ... -exec rm` | Block |
| `find ... -exec rm -rf` | Block |
| `find /tmp ... -delete` | Allow (safe location) |
| `find . ... -delete` (in cwd) | Warn |

```go
func AnalyzeFindCommand(parsed *ParsedCommand) (*RuleResult, error) {
    hasDelete := parsed.Flags["-delete"] || parsed.Flags["--delete"]
    hasExecRm := containsExecRm(parsed.Args)
    searchPath := extractSearchPath(parsed.Args)

    if hasDelete || hasExecRm {
        if isSafePath(searchPath) {
            return &RuleResult{Action: Allow}, nil
        }
        return &RuleResult{
            Action:  Block,
            Message: "find with -delete outside safe directories",
        }, nil
    }
    return &RuleResult{Action: Allow}, nil
}
```

---

### 5. Xargs/Parallel Chain Analysis

**Priority:** Medium
**Location:** New `internal/tools/rules_xargs.go`

Detect destructive commands piped through `xargs` or `parallel`:

| Pattern | Action |
|---------|--------|
| `... \| xargs rm -rf` | Block |
| `... \| parallel rm` | Block |
| `... \| xargs -I {} rm -rf {}` | Block |
| `xargs echo` | Allow |

```go
func AnalyzePipelineCommand(cmd string) (*RuleResult, error) {
    segments := splitPipeline(cmd)
    for _, seg := range segments {
        if isXargsOrParallel(seg) {
            innerCmd := extractXargsCommand(seg)
            result := analyzeCommand(innerCmd)
            if result.Action == Block {
                return &RuleResult{
                    Action:  Block,
                    Message: fmt.Sprintf("destructive command in %s pipeline", seg.Binary),
                }, nil
            }
        }
    }
    return &RuleResult{Action: Allow}, nil
}
```

---

### 6. Context-Aware rm -rf

**Priority:** Medium
**Location:** `internal/tools/rules_rm.go`

Current implementation blocks all `rm -rf`. Enhance with context awareness:

| Target | Action |
|--------|--------|
| `/` or `/*` | Block (always) |
| `~` or `$HOME` | Block (always) |
| Outside workspace | Block or prompt |
| `/tmp/*`, `/var/tmp/*`, `$TMPDIR/*` | Allow |
| Within workspace cwd | Allow |
| Workspace root itself | Block |

```go
func AnalyzeRmCommand(parsed *ParsedCommand, ctx *SafetyContext) (*RuleResult, error) {
    isRecursiveForce := parsed.Flags["r"] && parsed.Flags["f"]

    for _, target := range parsed.Args {
        resolved := resolvePath(target, ctx.WorkingDir)

        if isSystemCritical(resolved) {
            return block("rm -rf on system critical path: %s", resolved)
        }

        if isTempDirectory(resolved) {
            continue // allowed
        }

        if !isWithinWorkspace(resolved, ctx.Workspace) {
            if ctx.PathSafetyMode == "block" {
                return block("rm -rf outside workspace: %s", resolved)
            }
            return prompt("rm -rf outside workspace: %s", resolved)
        }

        if resolved == ctx.Workspace {
            return block("rm -rf on workspace root")
        }
    }

    return allow()
}
```

---

### 7. Strict Mode (Fail-Closed)

**Priority:** Medium
**Location:** `internal/config/config.go`, `internal/tools/shell.go`

Add configuration option for strict mode that fails closed on:
- Malformed JSON in tool calls
- Unterminated quotes in commands
- Unparseable shell syntax
- Unknown/suspicious patterns

```yaml
safety:
  strict_mode: true  # Fail-closed on unparseable input
```

```go
func (s *SafetyChecker) CheckCommand(cmd string) (*RuleResult, error) {
    parsed, err := ParseCommand(cmd)
    if err != nil {
        if s.config.Safety.StrictMode {
            return &RuleResult{
                Action:  Block,
                Message: fmt.Sprintf("strict mode: failed to parse command: %v", err),
            }, nil
        }
        // Non-strict: log warning and allow
        s.logger.Warn("failed to parse command, allowing", "cmd", cmd, "err", err)
    }
    // ... continue analysis
}
```

---

### 8. Paranoid Mode

**Priority:** Low
**Location:** `internal/config/config.go`, rules modules

Optional aggressive restriction mode:

```yaml
safety:
  paranoid_mode: true
```

**Paranoid mode restrictions:**
- Block ALL `rm -rf` except in `/tmp`
- Block ALL interpreter one-liners (`python -c`, `node -e`, etc.)
- Block ALL network commands
- Block ALL package manager commands
- Require explicit approval for any git operation that modifies history
- Block `eval`, `exec`, and dynamic execution patterns

```go
type SafetyConfig struct {
    StrictMode   bool `yaml:"strict_mode"`
    ParanoidMode bool `yaml:"paranoid_mode"`
}

func (r *RmRule) Analyze(parsed *ParsedCommand, ctx *SafetyContext) *RuleResult {
    if ctx.Config.Safety.ParanoidMode {
        if !isTempPath(target) {
            return block("paranoid mode: rm -rf only allowed in temp directories")
        }
    }
    // ... normal analysis
}
```

---

### 9. Audit Logging

**Priority:** Medium
**Location:** New `internal/safety/audit.go`

Log all blocked commands for security review:

**Log location:** `~/.kvit-coder/safety-logs/<session_id>.jsonl`

**Log format:**
```json
{
  "timestamp": "2025-01-15T10:30:00Z",
  "session_id": "abc123",
  "command": "rm -rf /important",
  "action": "blocked",
  "rule": "rm_critical_path",
  "message": "rm -rf on system critical path",
  "working_dir": "/home/user/project",
  "redacted_fields": ["env.API_KEY"]
}
```

```go
type AuditLogger struct {
    sessionID string
    logFile   *os.File
    redactor  *SecretRedactor
}

func (a *AuditLogger) LogBlocked(cmd string, result *RuleResult) error {
    entry := &AuditEntry{
        Timestamp:  time.Now().UTC(),
        SessionID:  a.sessionID,
        Command:    a.redactor.Redact(cmd),
        Action:     "blocked",
        Rule:       result.Rule,
        Message:    result.Message,
        WorkingDir: a.workingDir,
    }
    return a.write(entry)
}
```

---

### 10. Secret Redaction

**Priority:** Medium
**Location:** New `internal/safety/redactor.go`

Automatically redact sensitive data from logs and error messages:

**Patterns to redact:**
- API keys: `sk-...`, `pk_...`, `AKIA...`
- Tokens: `ghp_...`, `gho_...`, `Bearer ...`
- Passwords in URLs: `https://user:pass@host`
- Environment variables: `API_KEY=...`, `SECRET=...`
- Private keys: `-----BEGIN ... PRIVATE KEY-----`

```go
type SecretRedactor struct {
    patterns []*regexp.Regexp
}

var defaultPatterns = []string{
    `(?i)(api[_-]?key|apikey|secret|password|token|bearer)\s*[:=]\s*['"]?[\w\-\.]+['"]?`,
    `sk-[a-zA-Z0-9]{20,}`,
    `ghp_[a-zA-Z0-9]{36}`,
    `AKIA[A-Z0-9]{16}`,
    `-----BEGIN [A-Z ]+ PRIVATE KEY-----`,
    `https?://[^:]+:[^@]+@`,
}

func (r *SecretRedactor) Redact(input string) string {
    result := input
    for _, pattern := range r.patterns {
        result = pattern.ReplaceAllString(result, "[REDACTED]")
    }
    return result
}
```

---

## Implementation Priority

### Phase 1: Critical Security (Immediate)

1. **Git destructive operation blocking** - High data loss risk
2. **Shell wrapper detection** - Bypass prevention
3. **Semantic command parser** - Foundation for other rules

### Phase 2: Enhanced Protection (Short-term)

4. **Find-delete pattern blocking**
5. **Xargs/parallel chain analysis**
6. **Context-aware rm -rf**
7. **Strict mode**

### Phase 3: Observability & Advanced (Medium-term)

8. **Audit logging**
9. **Secret redaction**
10. **Paranoid mode**

---

## File Structure

```
internal/
├── safety/
│   ├── checker.go        # Main safety checker orchestration
│   ├── parser.go         # Semantic command parser
│   ├── audit.go          # Audit logging
│   ├── redactor.go       # Secret redaction
│   └── rules/
│       ├── rule.go       # Rule interface and types
│       ├── git.go        # Git operation rules
│       ├── rm.go         # rm command rules
│       ├── find.go       # find command rules
│       └── xargs.go      # xargs/parallel rules
```

---

## Configuration Schema Addition

```yaml
safety:
  # Existing
  path_safety_mode: "ask_once"

  # New options
  strict_mode: false          # Fail-closed on parse errors
  paranoid_mode: false        # Aggressive restrictions

  audit:
    enabled: true
    log_dir: "~/.kvit-coder/safety-logs"
    redact_secrets: true
    retention_days: 30

  git:
    block_push: true            # All push done by user
    block_hard_reset: true
    block_checkout_discard: true

  rm:
    allow_in_temp: true
    allow_in_workspace_cwd: true
    block_workspace_root: true

  interpreters:
    block_one_liners: false   # true in paranoid mode
    allowed: ["python", "node"]  # whitelist if blocking
```

---

## Testing Strategy

### Unit Tests

- Command parser edge cases (quotes, escapes, unicode)
- Each rule module with positive/negative cases
- Secret redaction patterns
- Path resolution in various contexts

### Integration Tests

- End-to-end command blocking
- Shell wrapper recursion limits
- Audit log rotation
- Configuration precedence

### Fuzz Testing

- Command parser with random/malformed input
- Secret redaction with adversarial patterns

---

## Migration Path

1. Add new safety package alongside existing code
2. Implement new rules without changing current behavior
3. Add configuration options (disabled by default)
4. Enable features incrementally with opt-in flags
5. Eventually make enhanced features default
6. Deprecate old simple blocklist approach

---

## References

- [claude-code-safety-net](https://github.com/kenryu42/claude-code-safety-net)
- Current implementation: `internal/tools/shell.go:317-424`
- Current path safety: `internal/config/permissions.go`, `internal/tools/path_utils.go`
