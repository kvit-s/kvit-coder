package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultConfigName is the file the binaries look for when nobody names one
// with -config.
const DefaultConfigName = "config.yaml"

// ConfigPathEnv names a config file for every kvit-coder run in the shell that
// exports it. A -config flag still wins over it.
const ConfigPathEnv = "KVIT_CODER_CONFIG"

// ResolvePath finds the configuration file to load. An explicit path is
// returned untouched, so a bad -config fails with the name the user typed
// rather than silently falling back to another file. Otherwise it looks, in
// order, at $KVIT_CODER_CONFIG, config.yaml in the current directory,
// ~/.kvit-coder/config.yaml, and config.yaml beside the binary. The last of
// those is what lets the command run from any directory: a symlink on $PATH
// resolves to the checkout, and the checkout's own config.yaml is found there.
//
// The second return value is a note for the user about a candidate that was
// passed over — empty when there is nothing to say.
func ResolvePath(explicit string) (string, string, error) {
	return ResolveNamed(explicit, DefaultConfigName)
}

// ResolveNamed is ResolvePath for a config file under some other name, which is
// what the benchmark modes use when they look for config-<suffix>.yaml.
func ResolveNamed(explicit, name string) (string, string, error) {
	if explicit != "" {
		return explicit, "", nil
	}
	if fromEnv := os.Getenv(ConfigPathEnv); fromEnv != "" {
		return fromEnv, "", nil
	}

	var (
		tried []string
		note  string
	)

	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, name)
		tried = append(tried, candidate)
		exists, ours := inspectCandidate(candidate, name)
		switch {
		case exists && ours:
			return candidate, "", nil
		case exists:
			// config.yaml is a popular file name. Loading a Hugo site's or a
			// Docker Compose project's copy would half-work and then fail at
			// the first request, so say what was skipped and keep looking.
			note = fmt.Sprintf("ignoring %s: it has no llm: or models: section, so it is not a kvit-coder config", candidate)
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, ".kvit-coder", name)
		tried = append(tried, candidate)
		if isFile(candidate) {
			return candidate, note, nil
		}
	}

	if exe, err := os.Executable(); err == nil {
		// Resolve the symlink first: the binaries are normally reached through
		// one in ~/.local/bin, and the config sits next to the real file.
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		candidate := filepath.Join(filepath.Dir(exe), name)
		tried = append(tried, candidate)
		if isFile(candidate) {
			return candidate, note, nil
		}
	}

	return "", note, fmt.Errorf("no config file found. Looked for %s. Name one with -config or $%s",
		strings.Join(tried, ", "), ConfigPathEnv)
}

// inspectCandidate reports whether the file is there and, for a file named
// config.yaml, whether it is a kvit-coder configuration rather than some other
// program's file of the same name. A config under any other name is taken at
// face value.
func inspectCandidate(path, name string) (exists, ours bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}
	if name != DefaultConfigName {
		return true, true
	}
	var top map[string]yaml.Node
	if err := yaml.Unmarshal(data, &top); err != nil {
		return true, false
	}
	_, hasLLM := top["llm"]
	_, hasModels := top["models"]
	return true, hasLLM || hasModels
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// ExpandHome turns a leading ~/ into the user's home directory.
func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
