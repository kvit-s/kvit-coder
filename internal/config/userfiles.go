package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Two files beside the config in ~/.kvit-coder hold what kvit-coder-ui saves
// when a model is set up from inside it, so that nobody has to edit
// config.yaml to add a model or export a key in a shell profile
// (spec/configing.md). Both are read here, by Load, because the agent is a new
// process every turn and finds its model by reading the configuration again:
// a row the front end kept only in memory would be refused by the agent.
// config.yaml itself is only ever written by a person.
const (
	// SavedModelsName holds model rows in the same format as the models: list
	// of config.yaml. Load appends them after the rows of the config file.
	SavedModelsName = "models.yaml"

	// CredentialsName holds API keys as a JSON object from environment
	// variable name to key, such as {"OPENCODE_API_KEY": "sk-..."}. A row's
	// api_key_env names the variable; the saved value is used when the
	// variable is not set. Keeping keys out of the environment matters: every
	// command the agent's Shell tool runs inherits the environment, so a key
	// placed there could be printed by any command the model chooses to run.
	CredentialsName = "credentials.json"
)

// LoadOptions changes what Load reads besides the config file itself.
type LoadOptions struct {
	// SkipSavedModels leaves out the rows in ~/.kvit-coder/models.yaml. The
	// benchmark modes set it, so a run does not depend on the models someone
	// set up on the machine it runs on.
	SkipSavedModels bool
}

// UserDir is ~/.kvit-coder (%USERPROFILE%\.kvit-coder on Windows), where the
// sessions, the update-check record and the files above live.
func UserDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kvit-coder"), nil
}

// SavedModelsPath is ~/.kvit-coder/models.yaml.
func SavedModelsPath() (string, error) {
	dir, err := UserDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, SavedModelsName), nil
}

// CredentialsPath is ~/.kvit-coder/credentials.json.
func CredentialsPath() (string, error) {
	dir, err := UserDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, CredentialsName), nil
}

// savedModelsFile is the whole of models.yaml: a models: list and nothing
// else. It is decoded with unknown keys refused, so an llm: block copied in
// from config.yaml, or a misspelt row key, fails at startup with the file's
// name rather than being ignored.
type savedModelsFile struct {
	Models []ModelEntry `yaml:"models"`
}

// readSavedModels returns the rows in models.yaml, or none when the file does
// not exist.
func readSavedModels(path string) ([]ModelEntry, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var file savedModelsFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&file); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return file.Models, nil
}

// The example configuration the installers seeded until October 2026 named
// this model at this address as a placeholder to replace. Nothing serves it,
// so a config that still has both is treated as naming no model: kvit-coder-ui
// then offers :setup, and a model saved there becomes the first row instead of
// coming after a model that does not exist.
const (
	examplePlaceholderModel   = "your-model"
	examplePlaceholderBaseURL = "http://localhost:8080/v1"
)

// dropExamplePlaceholder forgets the example's placeholder model.
func (c *Config) dropExamplePlaceholder() {
	if len(c.Models) == 0 && c.LLM.Model == examplePlaceholderModel && c.LLM.BaseURL == examplePlaceholderBaseURL {
		c.LLM.Model = ""
	}
}

// mergeSavedModels appends the rows of models.yaml after the config file's
// own models: rows, so every :mN number the config file gives keeps its
// meaning. A saved row whose id the config file already uses is left out:
// what a person wrote by hand takes precedence over what a program saved.
//
// A config file with no models: list but a model in its llm: block has one
// row today, built from that block by ModelList, and ModelList stops building
// it once any row exists. Saving a model from the front end must not make
// that working model disappear, so in that case the llm: row is written into
// the list first, exactly as ModelList would have built it, and the saved
// rows follow it.
func (c *Config) mergeSavedModels(path string) error {
	saved, err := readSavedModels(path)
	if err != nil || len(saved) == 0 {
		return err
	}
	if len(c.Models) == 0 && c.LLM.Model != "" {
		c.Models = c.ModelList()
	}
	taken := make(map[string]bool, len(c.Models))
	for _, e := range c.Models {
		taken[strings.ToLower(e.ID)] = true
	}
	c.savedModelsStart = len(c.Models)
	c.savedModelsPath = path
	for _, e := range saved {
		if e.ID != "" && taken[strings.ToLower(e.ID)] {
			continue
		}
		c.Models = append(c.Models, e)
	}
	return nil
}

// FromSavedModels reports whether row i of the model list came from
// models.yaml rather than from the config file.
func (c *Config) FromSavedModels(i int) bool {
	return c.savedModelsPath != "" && i >= c.savedModelsStart && i < len(c.Models)
}

// rowSource names where row i of the model list was written, for errors:
// "<config path>: models entry 3", or the same for models.yaml, counted
// within that file.
func (c *Config) rowSource(configPath string, i int) string {
	if c.FromSavedModels(i) {
		return fmt.Sprintf("%s: models entry %d", c.savedModelsPath, i-c.savedModelsStart+1)
	}
	return fmt.Sprintf("%s: models entry %d", configPath, i+1)
}

// readCredentials returns the keys in credentials.json, or none when the file
// does not exist.
func readCredentials() (map[string]string, error) {
	path, err := CredentialsPath()
	if err != nil {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	keys := map[string]string{}
	if len(bytes.TrimSpace(data)) == 0 {
		return keys, nil
	}
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return keys, nil
}

// SavedKey is the key credentials.json holds under an environment variable
// name, or "" when there is none. The file is read on every call: it is a few
// hundred bytes, the agent asks once or twice per turn, and reading it fresh
// means a key saved by one front end is seen by the next turn of another.
// An unreadable or malformed file reads as empty here; Load reports it.
func SavedKey(envName string) string {
	if envName == "" {
		return ""
	}
	keys, err := readCredentials()
	if err != nil {
		return ""
	}
	return keys[envName]
}

// LookupKey resolves an api_key_env name: the environment variable when it is
// set, else the key saved under that name in credentials.json.
func LookupKey(envName string) string {
	if envName == "" {
		return ""
	}
	if key := os.Getenv(envName); key != "" {
		return key
	}
	return SavedKey(envName)
}

// savedModelsHeader opens every models.yaml kvit-coder-ui writes. The file is
// rewritten whole on each save, so this is the only comment it keeps.
const savedModelsHeader = `# Written by kvit-coder-ui (:setup, :models). These rows are added after the
# models: list of config.yaml, and a row there with the same id takes
# precedence. Editing by hand is fine; kvit-coder-ui rewrites the whole file
# the next time it saves, keeping the rows and dropping other comments.
`

// ReadSavedModels returns the rows in ~/.kvit-coder/models.yaml, or none when
// the file does not exist.
func ReadSavedModels() ([]ModelEntry, error) {
	path, err := SavedModelsPath()
	if err != nil {
		return nil, err
	}
	return readSavedModels(path)
}

// WriteSavedModels replaces ~/.kvit-coder/models.yaml with these rows.
func WriteSavedModels(rows []ModelEntry) error {
	path, err := SavedModelsPath()
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString(savedModelsHeader)
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if rows == nil {
		rows = []ModelEntry{}
	}
	if err := enc.Encode(savedModelsFile{Models: rows}); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return writeFileAtomic(path, buf.Bytes(), 0o644)
}

// AppendSavedModels adds rows at the end of ~/.kvit-coder/models.yaml. Ids are
// the caller's to make unique; Load leaves out a saved row whose id is taken.
func AppendSavedModels(rows ...ModelEntry) error {
	existing, err := ReadSavedModels()
	if err != nil {
		return err
	}
	return WriteSavedModels(append(existing, rows...))
}

// RemoveSavedModel deletes the row with this id from ~/.kvit-coder/models.yaml
// and reports whether there was one.
func RemoveSavedModel(id string) (bool, error) {
	rows, err := ReadSavedModels()
	if err != nil {
		return false, err
	}
	kept := rows[:0]
	removed := false
	for _, e := range rows {
		if strings.EqualFold(e.ID, id) {
			removed = true
			continue
		}
		kept = append(kept, e)
	}
	if !removed {
		return false, nil
	}
	return true, WriteSavedModels(kept)
}

// SaveCredential stores key under an environment variable name in
// ~/.kvit-coder/credentials.json, which is written readable by its owner only.
func SaveCredential(envName, key string) error {
	if envName == "" {
		return errors.New("no variable name to save the key under")
	}
	keys, err := readCredentials()
	if err != nil {
		return err
	}
	if keys == nil {
		keys = map[string]string{}
	}
	keys[envName] = key
	return writeCredentials(keys)
}

// DeleteCredential removes the key saved under an environment variable name
// and reports whether there was one.
func DeleteCredential(envName string) (bool, error) {
	keys, err := readCredentials()
	if err != nil {
		return false, err
	}
	if _, ok := keys[envName]; !ok {
		return false, nil
	}
	delete(keys, envName)
	return true, writeCredentials(keys)
}

func writeCredentials(keys map[string]string) error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'), 0o600)
}

// writeFileAtomic writes data to a temporary file beside path and renames it
// over path, so a reader, or another kvit-coder-ui saving at the same moment,
// never sees half a file. The directory is created readable by its owner only
// when it does not exist yet.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name) // a no-op once the rename has happened
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
