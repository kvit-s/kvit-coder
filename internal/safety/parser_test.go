package safety

import (
	"testing"
)

func TestParseCommand_Basic(t *testing.T) {
	tests := []struct {
		name       string
		cmd        string
		binary     string
		subcommand string
		flags      map[string]bool
		args       []string
	}{
		{
			name:       "simple ls",
			cmd:        "ls",
			binary:     "ls",
			subcommand: "",
			flags:      map[string]bool{},
			args:       []string{},
		},
		{
			name:       "ls with flags",
			cmd:        "ls -la",
			binary:     "ls",
			subcommand: "",
			flags:      map[string]bool{"l": true, "a": true},
			args:       []string{},
		},
		{
			name:       "rm with combined flags",
			cmd:        "rm -rf /path",
			binary:     "rm",
			subcommand: "/path",
			flags:      map[string]bool{"r": true, "f": true},
			args:       []string{"/path"},
		},
		{
			name:       "rm with separate flags",
			cmd:        "rm -r -f /path",
			binary:     "rm",
			subcommand: "/path",
			flags:      map[string]bool{"r": true, "f": true},
			args:       []string{"/path"},
		},
		{
			name:       "git push with long flag",
			cmd:        "git push --force origin main",
			binary:     "git",
			subcommand: "push",
			flags:      map[string]bool{"force": true},
			args:       []string{"push", "origin", "main"},
		},
		{
			name:       "git push with short flag",
			cmd:        "git push -f origin main",
			binary:     "git",
			subcommand: "push",
			flags:      map[string]bool{"f": true},
			args:       []string{"push", "origin", "main"},
		},
		{
			name:       "git reset hard",
			cmd:        "git reset --hard HEAD~1",
			binary:     "git",
			subcommand: "reset",
			flags:      map[string]bool{"hard": true},
			args:       []string{"reset", "HEAD~1"},
		},
		{
			name:       "full path binary",
			cmd:        "/usr/bin/git status",
			binary:     "git",
			subcommand: "status",
			flags:      map[string]bool{},
			args:       []string{"status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseCommand(tt.cmd)
			if err != nil {
				t.Fatalf("ParseCommand failed: %v", err)
			}

			if parsed.Binary != tt.binary {
				t.Errorf("Binary = %q, want %q", parsed.Binary, tt.binary)
			}

			if parsed.Subcommand != tt.subcommand {
				t.Errorf("Subcommand = %q, want %q", parsed.Subcommand, tt.subcommand)
			}

			for flag := range tt.flags {
				if !parsed.Flags[flag] {
					t.Errorf("Expected flag %q to be set", flag)
				}
			}

			if len(parsed.Args) != len(tt.args) {
				t.Errorf("Args count = %d, want %d", len(parsed.Args), len(tt.args))
			} else {
				for i, arg := range tt.args {
					if parsed.Args[i] != arg {
						t.Errorf("Args[%d] = %q, want %q", i, parsed.Args[i], arg)
					}
				}
			}
		})
	}
}

func TestParseCommand_Quotes(t *testing.T) {
	tests := []struct {
		name   string
		cmd    string
		binary string
		args   []string
	}{
		{
			name:   "double quoted path",
			cmd:    `rm "file with spaces.txt"`,
			binary: "rm",
			args:   []string{"file with spaces.txt"},
		},
		{
			name:   "single quoted path",
			cmd:    `rm 'file with spaces.txt'`,
			binary: "rm",
			args:   []string{"file with spaces.txt"},
		},
		{
			name:   "mixed quotes",
			cmd:    `echo "it's working"`,
			binary: "echo",
			args:   []string{"it's working"},
		},
		{
			name:   "escaped quote",
			cmd:    `echo "test \"quoted\""`,
			binary: "echo",
			args:   []string{`test "quoted"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseCommand(tt.cmd)
			if err != nil {
				t.Fatalf("ParseCommand failed: %v", err)
			}

			if parsed.Binary != tt.binary {
				t.Errorf("Binary = %q, want %q", parsed.Binary, tt.binary)
			}

			if len(parsed.Args) != len(tt.args) {
				t.Errorf("Args = %v, want %v", parsed.Args, tt.args)
			} else {
				for i, arg := range tt.args {
					if parsed.Args[i] != arg {
						t.Errorf("Args[%d] = %q, want %q", i, parsed.Args[i], arg)
					}
				}
			}
		})
	}
}

func TestParseCommand_Pipelines(t *testing.T) {
	tests := []struct {
		name      string
		cmd       string
		binary    string
		pipeCount int
		pipeBins  []string
	}{
		{
			name:      "simple pipe",
			cmd:       "find . | xargs rm",
			binary:    "find",
			pipeCount: 1,
			pipeBins:  []string{"xargs"},
		},
		{
			name:      "multiple pipes",
			cmd:       "cat file | grep pattern | wc -l",
			binary:    "cat",
			pipeCount: 2,
			pipeBins:  []string{"grep", "wc"},
		},
		{
			name:      "pipe with quoted string",
			cmd:       `echo "hello | world" | cat`,
			binary:    "echo",
			pipeCount: 1,
			pipeBins:  []string{"cat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseCommand(tt.cmd)
			if err != nil {
				t.Fatalf("ParseCommand failed: %v", err)
			}

			if parsed.Binary != tt.binary {
				t.Errorf("Binary = %q, want %q", parsed.Binary, tt.binary)
			}

			if len(parsed.Pipes) != tt.pipeCount {
				t.Errorf("Pipe count = %d, want %d", len(parsed.Pipes), tt.pipeCount)
			}

			for i, pipeBin := range tt.pipeBins {
				if i < len(parsed.Pipes) && parsed.Pipes[i].Binary != pipeBin {
					t.Errorf("Pipes[%d].Binary = %q, want %q", i, parsed.Pipes[i].Binary, pipeBin)
				}
			}
		})
	}
}

func TestParseCommand_BashC(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		binary  string
		flagArg string
	}{
		{
			name:    "bash -c with single quotes",
			cmd:     "bash -c 'rm -rf /'",
			binary:  "bash",
			flagArg: "rm -rf /",
		},
		{
			name:    "bash -c with double quotes",
			cmd:     `bash -c "rm -rf /"`,
			binary:  "bash",
			flagArg: "rm -rf /",
		},
		{
			name:    "sh -c",
			cmd:     "sh -c 'git reset --hard'",
			binary:  "sh",
			flagArg: "git reset --hard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseCommand(tt.cmd)
			if err != nil {
				t.Fatalf("ParseCommand failed: %v", err)
			}

			if parsed.Binary != tt.binary {
				t.Errorf("Binary = %q, want %q", parsed.Binary, tt.binary)
			}

			if !parsed.Flags["c"] {
				t.Error("Expected -c flag to be set")
			}

			if parsed.FlagArgs["c"] != tt.flagArg {
				t.Errorf("FlagArgs[c] = %q, want %q", parsed.FlagArgs["c"], tt.flagArg)
			}
		})
	}
}

func TestParseCommand_GitCheckoutDoubleDash(t *testing.T) {
	cmd := "git checkout -- file.txt"
	parsed, err := ParseCommand(cmd)
	if err != nil {
		t.Fatalf("ParseCommand failed: %v", err)
	}

	if parsed.Binary != "git" {
		t.Errorf("Binary = %q, want git", parsed.Binary)
	}

	if parsed.Subcommand != "checkout" {
		t.Errorf("Subcommand = %q, want checkout", parsed.Subcommand)
	}

	if !parsed.HasDoubleDash() {
		t.Error("Expected HasDoubleDash() to return true")
	}

	argsAfter := parsed.ArgsAfterDoubleDash()
	if len(argsAfter) != 1 || argsAfter[0] != "file.txt" {
		t.Errorf("ArgsAfterDoubleDash() = %v, want [file.txt]", argsAfter)
	}
}

func TestParseCommand_HasFlag(t *testing.T) {
	cmd := "rm -rf --verbose /path"
	parsed, err := ParseCommand(cmd)
	if err != nil {
		t.Fatalf("ParseCommand failed: %v", err)
	}

	// Test HasFlag with various formats
	if !parsed.HasFlag("r") {
		t.Error("Expected HasFlag(r) to return true")
	}
	if !parsed.HasFlag("-r") {
		t.Error("Expected HasFlag(-r) to return true")
	}
	if !parsed.HasFlag("verbose") {
		t.Error("Expected HasFlag(verbose) to return true")
	}
	if !parsed.HasFlag("--verbose") {
		t.Error("Expected HasFlag(--verbose) to return true")
	}
	if parsed.HasFlag("x") {
		t.Error("Expected HasFlag(x) to return false")
	}
}

func TestParseCommand_HasAnyDestructiveRmFlags(t *testing.T) {
	tests := []struct {
		cmd       string
		recursive bool
		force     bool
	}{
		{"rm -rf /path", true, true},
		{"rm -r /path", true, false},
		{"rm -f /path", false, true},
		{"rm /path", false, false},
		{"rm --recursive --force /path", true, true},
		{"rm -R /path", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			parsed, err := ParseCommand(tt.cmd)
			if err != nil {
				t.Fatalf("ParseCommand failed: %v", err)
			}

			r, f := parsed.HasAnyDestructiveRmFlags()
			if r != tt.recursive {
				t.Errorf("recursive = %v, want %v", r, tt.recursive)
			}
			if f != tt.force {
				t.Errorf("force = %v, want %v", f, tt.force)
			}
		})
	}
}

func TestParseCommand_Errors(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{"empty command", ""},
		{"unterminated single quote", "echo 'hello"},
		{"unterminated double quote", `echo "hello`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCommand(tt.cmd)
			if err == nil {
				t.Error("Expected error but got nil")
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		cmd    string
		tokens []string
	}{
		{"ls", []string{"ls"}},
		{"ls -la", []string{"ls", "-la"}},
		{"rm 'file name.txt'", []string{"rm", "file name.txt"}},
		{`echo "hello world"`, []string{"echo", "hello world"}},
		{"cmd arg1 arg2", []string{"cmd", "arg1", "arg2"}},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			tokens, err := tokenize(tt.cmd)
			if err != nil {
				t.Fatalf("tokenize failed: %v", err)
			}

			if len(tokens) != len(tt.tokens) {
				t.Errorf("token count = %d, want %d\ngot:  %v\nwant: %v",
					len(tokens), len(tt.tokens), tokens, tt.tokens)
			} else {
				for i, tok := range tt.tokens {
					if tokens[i] != tok {
						t.Errorf("token[%d] = %q, want %q", i, tokens[i], tok)
					}
				}
			}
		})
	}
}
