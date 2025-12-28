package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Manager handles checkpoint creation and restoration using a shadow git repository
type Manager struct {
	mu              sync.RWMutex
	sessionID       string
	workdir         string
	checkpointDir   string
	currentTurn     int
	enabled         bool
	excludePatterns []string
	maxFileSizeKB   int
}

// ExternalFileMapping tracks files outside the workdir
type ExternalFileMapping struct {
	OriginalPath     string `json:"original_path"`
	FirstTrackedTurn int    `json:"first_tracked_turn"`
}

// TurnInfo contains information about a checkpoint turn
type TurnInfo struct {
	Turn         int      `json:"turn"`
	FilesChanged []string `json:"files_changed"`
	IsRestore    bool     `json:"is_restore,omitempty"`
	RestoredTo   int      `json:"restored_to,omitempty"`
}

// NewManager creates a new checkpoint manager
func NewManager(sessionID, workdir string, excludePatterns []string, maxFileSizeKB int) (*Manager, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID cannot be empty")
	}
	if workdir == "" {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Convert to absolute path
	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve workdir: %w", err)
	}

	// Set defaults
	if len(excludePatterns) == 0 {
		excludePatterns = []string{
			".git/*",
			"node_modules/*",
			"__pycache__/*",
			"*.pyc",
			".venv/*",
			"venv/*",
			"vendor/*",
			".idea/*",
			".vscode/*",
			"*.log",
		}
	}
	if maxFileSizeKB == 0 {
		maxFileSizeKB = 1024 // 1MB default
	}

	checkpointDir := filepath.Join(os.TempDir(), fmt.Sprintf("go-coder-checkpoints-%s", sessionID))

	m := &Manager{
		sessionID:       sessionID,
		workdir:         absWorkdir,
		checkpointDir:   checkpointDir,
		currentTurn:     0,
		enabled:         true,
		excludePatterns: excludePatterns,
		maxFileSizeKB:   maxFileSizeKB,
	}

	return m, nil
}

// Initialize sets up the shadow git repository
func (m *Manager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enabled {
		return nil
	}

	// Create checkpoint directory
	if err := os.MkdirAll(m.checkpointDir, 0755); err != nil {
		return fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	// Create external-files directory
	externalDir := filepath.Join(m.checkpointDir, "external-files")
	if err := os.MkdirAll(externalDir, 0755); err != nil {
		return fmt.Errorf("failed to create external-files directory: %w", err)
	}

	// Initialize external-files.json
	mappingFile := filepath.Join(m.checkpointDir, "external-files.json")
	if err := os.WriteFile(mappingFile, []byte("{}"), 0644); err != nil {
		return fmt.Errorf("failed to create external-files.json: %w", err)
	}

	// Initialize shadow git repo
	gitDir := filepath.Join(m.checkpointDir, ".git")
	cmd := exec.Command("git", "init", "--bare", gitDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to init shadow git: %w\nOutput: %s", err, output)
	}

	// Create initial commit (turn-0) with empty tree
	if err := m.createInitialCommit(); err != nil {
		return fmt.Errorf("failed to create initial commit: %w", err)
	}

	return nil
}

// createInitialCommit creates the turn-0 commit with empty tree
func (m *Manager) createInitialCommit() error {
	gitDir := filepath.Join(m.checkpointDir, ".git")

	// Create exclude file in the git info directory
	infoDir := filepath.Join(gitDir, "info")
	if err := os.MkdirAll(infoDir, 0755); err != nil {
		return fmt.Errorf("failed to create git info directory: %w", err)
	}

	// Write exclude patterns to .git/info/exclude
	excludeContent := strings.Join(m.excludePatterns, "\n")
	excludeFile := filepath.Join(infoDir, "exclude")
	if err := os.WriteFile(excludeFile, []byte(excludeContent), 0644); err != nil {
		return fmt.Errorf("failed to create exclude file: %w", err)
	}

	// Stage all current files to capture initial state
	cmd := exec.Command("git",
		"--git-dir="+gitDir,
		"--work-tree="+m.workdir,
		"add", "-A",
	)
	cmd.Dir = m.workdir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stage initial files: %w\nOutput: %s", err, output)
	}

	// Create initial commit
	cmd = exec.Command("git",
		"--git-dir="+gitDir,
		"--work-tree="+m.workdir,
		"commit", "--allow-empty", "-m", "turn-0",
	)
	cmd.Dir = m.workdir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=go-coder",
		"GIT_AUTHOR_EMAIL=go-coder@local",
		"GIT_COMMITTER_NAME=go-coder",
		"GIT_COMMITTER_EMAIL=go-coder@local",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create initial commit: %w\nOutput: %s", err, output)
	}

	// Tag it as turn-0
	cmd = exec.Command("git",
		"--git-dir="+gitDir,
		"tag", "turn-0",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to tag turn-0: %w\nOutput: %s", err, output)
	}

	return nil
}

// CurrentTurn returns the current turn number
func (m *Manager) CurrentTurn() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentTurn
}

// StartTurn increments the turn counter (called before processing tool calls)
func (m *Manager) StartTurn() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentTurn++
	return m.currentTurn
}

// EndTurn commits all changes to the shadow git (called after all tool calls complete)
func (m *Manager) EndTurn() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enabled {
		return nil
	}

	return m.commitTurn(m.currentTurn, false, 0)
}

// commitTurn stages and commits all changes for a turn
func (m *Manager) commitTurn(turnNum int, isRestore bool, restoredTo int) error {
	gitDir := filepath.Join(m.checkpointDir, ".git")

	// Stage all changes (exclude patterns are in .git/info/exclude)
	cmd := exec.Command("git",
		"--git-dir="+gitDir,
		"--work-tree="+m.workdir,
		"add", "-A",
	)
	cmd.Dir = m.workdir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stage changes: %w\nOutput: %s", err, output)
	}

	// Also stage external files and mapping
	cmd = exec.Command("git",
		"--git-dir="+gitDir,
		"--work-tree="+m.checkpointDir,
		"add", "-Af",
		"external-files",
		"external-files.json",
	)
	cmd.Dir = m.checkpointDir
	// Ignore errors - these files might not exist yet
	cmd.CombinedOutput()

	// Create commit message
	commitMsg := fmt.Sprintf("turn-%d", turnNum)
	if isRestore {
		commitMsg = fmt.Sprintf("turn-%d (restored to turn-%d)", turnNum, restoredTo)
	}

	// Commit
	cmd = exec.Command("git",
		"--git-dir="+gitDir,
		"--work-tree="+m.workdir,
		"commit", "--allow-empty", "-m", commitMsg,
	)
	cmd.Dir = m.workdir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=go-coder",
		"GIT_AUTHOR_EMAIL=go-coder@local",
		"GIT_COMMITTER_NAME=go-coder",
		"GIT_COMMITTER_EMAIL=go-coder@local",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Check if it's just "nothing to commit"
		if !strings.Contains(string(output), "nothing to commit") {
			return fmt.Errorf("failed to commit: %w\nOutput: %s", err, output)
		}
	}

	// Tag this turn
	tagName := fmt.Sprintf("turn-%d", turnNum)
	cmd = exec.Command("git",
		"--git-dir="+gitDir,
		"tag", "-f", tagName,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to tag %s: %w\nOutput: %s", tagName, err, output)
	}

	return nil
}

// IsExternalPath checks if a path is outside the workdir
func (m *Manager) IsExternalPath(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Check if path is outside workdir
	rel, err := filepath.Rel(m.workdir, absPath)
	if err != nil {
		return true // Can't compute relative path, assume external
	}

	return strings.HasPrefix(rel, "..")
}

// TrackExternalFile copies a file from outside workdir to external-files/
func (m *Manager) TrackExternalFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enabled {
		return nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Load current mappings
	mappings, err := m.loadExternalMappings()
	if err != nil {
		return fmt.Errorf("failed to load mappings: %w", err)
	}

	// Check if already tracked
	for _, mapping := range mappings {
		if mapping.OriginalPath == absPath {
			// Already tracked, just copy current state
			key := m.getExternalFileKey(absPath, mappings)
			return m.copyExternalFile(absPath, key)
		}
	}

	// Generate new key
	counter := len(mappings) + 1
	sanitized := strings.ReplaceAll(absPath, "/", "-")
	sanitized = strings.TrimPrefix(sanitized, "-")
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}
	key := fmt.Sprintf("%04d-%s", counter, sanitized)

	// Add to mappings
	mappings[key] = ExternalFileMapping{
		OriginalPath:     absPath,
		FirstTrackedTurn: m.currentTurn,
	}

	// Save mappings
	if err := m.saveExternalMappings(mappings); err != nil {
		return fmt.Errorf("failed to save mappings: %w", err)
	}

	// Copy file
	return m.copyExternalFile(absPath, key)
}

// getExternalFileKey finds the key for an already-tracked external file
func (m *Manager) getExternalFileKey(absPath string, mappings map[string]ExternalFileMapping) string {
	for key, mapping := range mappings {
		if mapping.OriginalPath == absPath {
			return key
		}
	}
	return ""
}

// copyExternalFile copies a file to external-files/
func (m *Manager) copyExternalFile(srcPath, key string) error {
	dstPath := filepath.Join(m.checkpointDir, "external-files", key)

	// Read source file (if it exists)
	content, err := os.ReadFile(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, create empty marker
			content = []byte{}
		} else {
			return fmt.Errorf("failed to read external file: %w", err)
		}
	}

	// Write to external-files
	return os.WriteFile(dstPath, content, 0644)
}

// loadExternalMappings loads the external-files.json
func (m *Manager) loadExternalMappings() (map[string]ExternalFileMapping, error) {
	mappingFile := filepath.Join(m.checkpointDir, "external-files.json")
	data, err := os.ReadFile(mappingFile)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]ExternalFileMapping), nil
		}
		return nil, err
	}

	var mappings map[string]ExternalFileMapping
	if err := json.Unmarshal(data, &mappings); err != nil {
		return nil, err
	}
	return mappings, nil
}

// saveExternalMappings saves the external-files.json
func (m *Manager) saveExternalMappings(mappings map[string]ExternalFileMapping) error {
	mappingFile := filepath.Join(m.checkpointDir, "external-files.json")
	data, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(mappingFile, data, 0644)
}

// Restore restores files to the state after a given turn
func (m *Manager) Restore(turnNum int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enabled {
		return nil, fmt.Errorf("checkpoints not enabled")
	}

	if turnNum < 0 || turnNum > m.currentTurn {
		return nil, fmt.Errorf("invalid turn number: %d (current turn: %d)", turnNum, m.currentTurn)
	}

	gitDir := filepath.Join(m.checkpointDir, ".git")
	tagName := fmt.Sprintf("turn-%d", turnNum)

	// First, get list of files that will change
	cmd := exec.Command("git",
		"--git-dir="+gitDir,
		"--work-tree="+m.workdir,
		"diff", "--name-only", tagName,
	)
	cmd.Dir = m.workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w\nOutput: %s", err, output)
	}

	changedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(changedFiles) == 1 && changedFiles[0] == "" {
		changedFiles = []string{}
	}

	// Checkout workdir files from the target turn
	cmd = exec.Command("git",
		"--git-dir="+gitDir,
		"--work-tree="+m.workdir,
		"checkout", tagName, "--", ".",
	)
	cmd.Dir = m.workdir
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to restore workdir: %w\nOutput: %s", err, output)
	}

	// Restore external files
	if err := m.restoreExternalFiles(turnNum); err != nil {
		return changedFiles, fmt.Errorf("workdir restored but external files failed: %w", err)
	}

	// Increment turn and commit the restore as a new checkpoint
	m.currentTurn++
	if err := m.commitTurn(m.currentTurn, true, turnNum); err != nil {
		return changedFiles, fmt.Errorf("restore succeeded but commit failed: %w", err)
	}

	return changedFiles, nil
}

// restoreExternalFiles restores external files from a turn
func (m *Manager) restoreExternalFiles(turnNum int) error {
	gitDir := filepath.Join(m.checkpointDir, ".git")
	tagName := fmt.Sprintf("turn-%d", turnNum)

	// Get external-files.json from that commit
	cmd := exec.Command("git",
		"--git-dir="+gitDir,
		"show", fmt.Sprintf("%s:external-files.json", tagName),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// No external files at that turn
		return nil
	}

	var mappings map[string]ExternalFileMapping
	if err := json.Unmarshal(output, &mappings); err != nil {
		return fmt.Errorf("failed to parse mappings: %w", err)
	}

	// Restore each external file
	for key, mapping := range mappings {
		// Get file content from that commit
		cmd := exec.Command("git",
			"--git-dir="+gitDir,
			"show", fmt.Sprintf("%s:external-files/%s", tagName, key),
		)
		content, err := cmd.CombinedOutput()
		if err != nil {
			continue // File might not exist at that turn
		}

		// Write back to original location
		if err := os.WriteFile(mapping.OriginalPath, content, 0644); err != nil {
			return fmt.Errorf("failed to restore %s: %w", mapping.OriginalPath, err)
		}
	}

	return nil
}

// Diff returns the diff between current state and a turn
func (m *Manager) Diff(turnNum int, path string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled {
		return "", fmt.Errorf("checkpoints not enabled")
	}

	if turnNum < 0 || turnNum > m.currentTurn {
		return "", fmt.Errorf("invalid turn number: %d (current turn: %d)", turnNum, m.currentTurn)
	}

	gitDir := filepath.Join(m.checkpointDir, ".git")
	tagName := fmt.Sprintf("turn-%d", turnNum)

	args := []string{
		"--git-dir=" + gitDir,
		"--work-tree=" + m.workdir,
		"diff", tagName,
	}

	if path != "" {
		args = append(args, "--", path)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = m.workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w\nOutput: %s", err, output)
	}

	return string(output), nil
}

// List returns information about all checkpoints
func (m *Manager) List() ([]TurnInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled {
		return nil, fmt.Errorf("checkpoints not enabled")
	}

	gitDir := filepath.Join(m.checkpointDir, ".git")
	var turns []TurnInfo

	for i := 1; i <= m.currentTurn; i++ {
		prevTag := fmt.Sprintf("turn-%d", i-1)
		currTag := fmt.Sprintf("turn-%d", i)

		// Get files changed between turns
		cmd := exec.Command("git",
			"--git-dir="+gitDir,
			"--work-tree="+m.workdir,
			"diff", "--name-only", prevTag, currTag,
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			continue
		}

		files := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(files) == 1 && files[0] == "" {
			files = []string{}
		}

		// Check if this was a restore by looking at commit message
		cmd = exec.Command("git",
			"--git-dir="+gitDir,
			"log", "-1", "--format=%s", currTag,
		)
		msgOutput, _ := cmd.CombinedOutput()
		msg := strings.TrimSpace(string(msgOutput))

		info := TurnInfo{
			Turn:         i,
			FilesChanged: files,
		}

		// Parse restore info from commit message
		if strings.Contains(msg, "restored to") {
			info.IsRestore = true
			// Extract restored turn number
			parts := strings.Split(msg, "restored to turn-")
			if len(parts) > 1 {
				numStr := strings.TrimSuffix(parts[1], ")")
				if num, err := strconv.Atoi(numStr); err == nil {
					info.RestoredTo = num
				}
			}
		}

		turns = append(turns, info)
	}

	// Sort by turn number
	sort.Slice(turns, func(i, j int) bool {
		return turns[i].Turn < turns[j].Turn
	})

	return turns, nil
}

// Cleanup removes the checkpoint directory
func (m *Manager) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.checkpointDir == "" {
		return nil
	}

	return os.RemoveAll(m.checkpointDir)
}

// Enabled returns whether checkpoints are enabled
func (m *Manager) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// SetEnabled enables or disables checkpoints
func (m *Manager) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
}

// Workdir returns the workspace directory
func (m *Manager) Workdir() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.workdir
}

// GetModifiedFiles returns a list of all files modified since turn-0 (session start)
// This uses git to find files that have been changed during this session
func (m *Manager) GetModifiedFiles() []string {
	if !m.enabled {
		return nil
	}

	gitDir := filepath.Join(m.checkpointDir, ".git")

	// Get list of files that differ from turn-0
	cmd := exec.Command("git",
		"--git-dir="+gitDir,
		"--work-tree="+m.workdir,
		"diff", "--name-only", "turn-0",
	)
	cmd.Dir = m.workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			// Convert relative path to absolute
			absPath := filepath.Join(m.workdir, line)
			files = append(files, absPath)
		}
	}

	sort.Strings(files)
	return files
}

// findSimilarFiles finds modified files that match the given filename
// Only returns files that can actually be restored (existed at turn-0)
func (m *Manager) findSimilarFiles(targetPath string) []string {
	modifiedFiles := m.GetModifiedFiles()
	if len(modifiedFiles) == 0 {
		return nil
	}

	gitDir := filepath.Join(m.checkpointDir, ".git")
	targetBase := filepath.Base(targetPath)
	targetBaseLower := strings.ToLower(targetBase)

	var matches []string
	for _, modifiedPath := range modifiedFiles {
		// Check if this file actually existed at turn-0 (can be restored)
		// Skip files that were created during the session
		relPath, err := filepath.Rel(m.workdir, modifiedPath)
		if err != nil || strings.HasPrefix(relPath, "..") {
			continue
		}

		// Check if file existed at turn-0
		cmd := exec.Command("git",
			"--git-dir="+gitDir,
			"cat-file", "-e", fmt.Sprintf("turn-0:%s", relPath),
		)
		if err := cmd.Run(); err != nil {
			// File didn't exist at turn-0, skip it
			continue
		}

		modifiedBase := filepath.Base(modifiedPath)
		modifiedBaseLower := strings.ToLower(modifiedBase)

		// Exact filename match (case-insensitive)
		if modifiedBaseLower == targetBaseLower {
			matches = append(matches, modifiedPath)
			continue
		}

		// Partial match: target is contained in modified filename or vice versa
		if strings.Contains(modifiedBaseLower, targetBaseLower) ||
			strings.Contains(targetBaseLower, modifiedBaseLower) {
			matches = append(matches, modifiedPath)
			continue
		}

		// Match if target path suffix matches (e.g., "migrations/serializer.py" matches "/testbed/django/db/migrations/serializer.py")
		if strings.HasSuffix(modifiedPath, targetPath) ||
			strings.HasSuffix(strings.ToLower(modifiedPath), strings.ToLower(targetPath)) {
			matches = append(matches, modifiedPath)
		}
	}

	sort.Strings(matches)
	return matches
}

// RestoreFile restores a single file to its state at turn 0 (initial state)
// Returns the content that was restored, or error if file wasn't tracked
func (m *Manager) RestoreFile(path string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enabled {
		return nil, fmt.Errorf("checkpoints not enabled")
	}

	// Normalize path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	// Make path relative to workdir for git
	relPath, err := filepath.Rel(m.workdir, absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get relative path: %w", err)
	}

	// Check if path is outside workdir
	if strings.HasPrefix(relPath, "..") {
		return nil, fmt.Errorf("cannot restore files outside workspace: %s", path)
	}

	gitDir := filepath.Join(m.checkpointDir, ".git")

	// Get file content from turn-0
	cmd := exec.Command("git",
		"--git-dir="+gitDir,
		"--work-tree="+m.workdir,
		"show", fmt.Sprintf("turn-0:%s", relPath),
	)
	content, err := cmd.CombinedOutput()
	if err != nil {
		// Check if the file didn't exist at turn-0
		outputStr := string(content)
		if strings.Contains(outputStr, "does not exist") ||
			strings.Contains(outputStr, "Path") ||
			strings.Contains(outputStr, "exists on disk, but not in") {
			errMsg := fmt.Sprintf("cannot restore '%s': this file was not present in the workspace at session start.\n\nfs.restore_file can only restore files that existed before this session began. Files created during this session have no original state to restore to.", path)

			// Find similar files that were edited during this session
			similarFiles := m.findSimilarFiles(path)
			if len(similarFiles) > 0 {
				errMsg += "\n\nFiles that CAN be restored (edited during this session):"
				for _, f := range similarFiles {
					errMsg += "\n  - " + f
				}
			}

			return nil, fmt.Errorf("%s", errMsg)
		}
		return nil, fmt.Errorf("failed to get original content: %w\nOutput: %s", err, content)
	}

	// Write the content back to the file
	if err := os.WriteFile(absPath, content, 0644); err != nil {
		return nil, fmt.Errorf("failed to write restored content: %w", err)
	}

	return content, nil
}
