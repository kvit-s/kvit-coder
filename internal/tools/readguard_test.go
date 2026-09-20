package tools

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readThroughTool calls Read the way the model does, with the workspace rooted
// at dir, and returns the result map.
func readThroughTool(t *testing.T, dir string, args map[string]any) map[string]any {
	t.Helper()

	cfg := newTestConfig()
	cfg.Workspace.Root = dir
	tool := NewReadFileTool(cfg, NewToolContext())

	encoded, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	result, err := tool.Call(context.Background(), encoded)
	if err != nil {
		t.Fatalf("Read %v: %v", args, err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Read returned %T, want map", result)
	}
	return m
}

func writeFileForGuard(t *testing.T, dir, name string, content []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// A file that is not text is named and refused, with something the model can
// act on, instead of being read as lines of replacement characters.
func TestReadNamesFilesItCannotRead(t *testing.T) {
	dir := t.TempDir()

	// A minimal ZIP: the end-of-central-directory record on its own is a valid
	// empty archive, and the local-file header is what a populated one starts
	// with. Both have to be recognized.
	zipBytes := append([]byte("PK\x03\x04"), bytes.Repeat([]byte{0}, 26)...)

	var gzipped bytes.Buffer
	zw := gzip.NewWriter(&gzipped)
	zw.Write([]byte("some content"))
	zw.Close()

	cases := []struct {
		name     string
		content  []byte
		wantType string
		wantHint string
	}{
		{"shot.png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), "PNG image", "ReadImage"},
		{"photo.jpg", []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00"), "JPEG image", "ReadImage"},
		{"icon.webp", []byte("RIFF\x24\x00\x00\x00WEBPVP8 "), "WebP image", "ReadImage"},
		{"bundle.zip", zipBytes, "ZIP archive", "unzip -l"},
		{"report.docx", zipBytes, "Word document (a ZIP container)", "word/document.xml"},
		{"package.whl", zipBytes, "Python wheel", "unzip -l"},
		{"logs.gz", gzipped.Bytes(), "gzip file", "tar -tzf"},
		{"app.db", append([]byte("SQLite format 3\x00"), bytes.Repeat([]byte{0}, 16)...), "SQLite database", "sqlite3"},
		{"libthing.so", []byte("\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00"), "compiled object or executable (ELF)", "objdump"},
		{"module.wasm", []byte("\x00asm\x01\x00\x00\x00"), "WebAssembly module", "wasm-objdump"},
		{"cached.pyc", []byte("\xcb\x0d\r\n\x00\x00\x00\x00compiled"), "compiled Python bytecode", "python3 -m dis"},
		{"notes.txt", append([]byte("\xff\xfeh\x00e\x00l\x00l\x00o\x00"), 0), "UTF-16 text (little endian)", "iconv"},
		{"mystery.dat", []byte("\x01\x02\x00\x03\x04nothing known here"), "binary file", "char_mode"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeFileForGuard(t, dir, tc.name, tc.content)
			got := readThroughTool(t, dir, map[string]any{"path": tc.name})

			if got["success"] != false {
				t.Fatalf("success = %v, want false (content: %v)", got["success"], got["content"])
			}
			if got["error"] != "not_text" {
				t.Errorf("error = %v, want not_text", got["error"])
			}
			if got["file_type"] != tc.wantType {
				t.Errorf("file_type = %v, want %q", got["file_type"], tc.wantType)
			}
			hint, _ := got["hint"].(string)
			if !strings.Contains(hint, tc.wantHint) {
				t.Errorf("hint = %q, want it to mention %q", hint, tc.wantHint)
			}
			if _, hasContent := got["content"]; hasContent {
				t.Error("a refused file still returned content")
			}
		})
	}
}

// The guard must not touch ordinary source files, which is the case that
// matters most: a false positive would make a file unreadable.
func TestReadStillReadsText(t *testing.T) {
	dir := t.TempDir()

	cases := map[string]string{
		"main.go":      "package main\n\nfunc main() {}\n",
		"data.json":    `{"cells": "this is not a notebook", "n": 1}`,
		"drawing.svg":  `<svg xmlns="http://www.w3.org/2000/svg"><rect width="1" height="1"/></svg>`,
		"README.md":    "# Title\n\nSome prose.\n",
		"table.csv":    "a,b,c\n1,2,3\n",
		"with-bom.txt": "\ufeffleading byte order mark\n",
		"empty.txt":    "",
		"script.sh":    "#!/bin/sh\necho hello\n",
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			writeFileForGuard(t, dir, name, []byte(content))
			got := readThroughTool(t, dir, map[string]any{"path": name})

			if got["success"] != true {
				t.Fatalf("success = %v, message = %v", got["success"], got["message"])
			}
			if content == "" {
				return
			}
			text, _ := got["content"].(string)
			firstLine := strings.SplitN(strings.TrimSpace(content), "\n", 2)[0]
			if !strings.Contains(text, strings.TrimPrefix(firstLine, "\ufeff")) {
				t.Errorf("content does not contain %q:\n%s", firstLine, text)
			}
		})
	}
}

// char_mode is the way to the bytes for a caller that means to have them.
func TestCharModeBypassesTheGuard(t *testing.T) {
	dir := t.TempDir()
	writeFileForGuard(t, dir, "libthing.so", []byte("\x7fELF\x02\x01\x01\x00padding here"))

	got := readThroughTool(t, dir, map[string]any{"path": "libthing.so", "char_mode": true})

	if got["success"] != true {
		t.Fatalf("success = %v, want the bytes: %v", got["success"], got["message"])
	}
	if _, hasContent := got["content"]; !hasContent {
		t.Error("char_mode returned no content")
	}
}

// A notebook reads as usual, and is told how to get its code without the
// base64 images that dominate an executed one.
func TestReadAdvisesOnNotebooks(t *testing.T) {
	dir := t.TempDir()
	notebook := `{
 "cells": [
  {"cell_type": "code", "source": ["print('hi')\n"], "outputs": []}
 ],
 "metadata": {},
 "nbformat": 4
}`
	writeFileForGuard(t, dir, "research.ipynb", []byte(notebook))

	got := readThroughTool(t, dir, map[string]any{"path": "research.ipynb"})

	if got["success"] != true {
		t.Fatalf("success = %v, want the notebook to be read", got["success"])
	}
	content, _ := got["content"].(string)
	if !strings.Contains(content, "print('hi')") {
		t.Errorf("notebook content is missing its source:\n%s", content)
	}
	hint, _ := got["notebook_hint"].(string)
	if !strings.Contains(hint, "jq") {
		t.Errorf("notebook_hint = %q, want it to offer the jq recipe", hint)
	}
}

// A file merely named .ipynb gets no notebook advice, because the advice would
// not work on it.
func TestNotebookAdviceNeedsANotebook(t *testing.T) {
	dir := t.TempDir()
	writeFileForGuard(t, dir, "fake.ipynb", []byte("{\"not\": \"a notebook\"}"))

	got := readThroughTool(t, dir, map[string]any{"path": "fake.ipynb"})

	if _, has := got["notebook_hint"]; has {
		t.Error("a file that is not a notebook was given notebook advice")
	}
}

func TestClassifyNotTextLeavesTextAlone(t *testing.T) {
	cases := []string{
		"package main\n",
		"#!/bin/sh\n",
		"<?xml version=\"1.0\"?><root/>",
		"",
		"plain words and nothing else",
	}
	for _, content := range cases {
		if kind, notText := classifyNotText("file.txt", []byte(content)); notText {
			t.Errorf("classifyNotText(%q) = %q, want it read as text", content, kind.Name)
		}
	}
}

func TestFormatFileSize(t *testing.T) {
	cases := []struct {
		size int64
		want string
	}{
		{0, "0 bytes"},
		{512, "512 bytes"},
		{2048, "2.0 KB"},
		{5 * 1 << 20, "5.0 MB"},
		{3 * 1 << 30, "3.0 GB"},
	}
	for _, tc := range cases {
		if got := formatFileSize(tc.size); got != tc.want {
			t.Errorf("formatFileSize(%d) = %q, want %q", tc.size, got, tc.want)
		}
	}
}
