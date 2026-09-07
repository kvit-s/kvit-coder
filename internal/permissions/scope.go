// Package permissions decides whether a shell command may run, from what the
// command actually is rather than from what its text looks like.
//
// The old check was a list of regular expressions over the command string. It
// could not tell a program being invoked from the same word appearing as a grep
// pattern without a hand-written boundary rule for each one, it could not see
// past the first segment of "git diff && rm -rf /", and it had no way to say
// "yes to this one" — a blocked command stayed blocked. Parsing the command
// gives all three: every simple command in it is found, wherever it sits, and
// each one can be named by a rule and granted on its own.
package permissions

import (
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Scope is one simple command found inside a command line: the program and the
// words handed to it. A command line has as many scopes as it has commands, so
// "git diff && rm -rf /" has two and each is judged on its own.
type Scope struct {
	// Program is what is being run, with any directory prefix removed:
	// /usr/bin/curl and ./curl are both "curl".
	Program string
	// Args are the words after the program, as written.
	Args []string
	// Literal is the whole simple command as written, which is what a grant
	// names when the command cannot safely be generalized.
	Literal string
	// Paths are the arguments that name a file or directory.
	Paths []string
	// Generalizable says whether a grant for this scope may be widened to
	// "<program> *". It is false inside a loop or a conditional, where the
	// arguments come from the surrounding construct rather than from the text
	// in front of you: granting "rm *" on the strength of "for f in *; do rm
	// $f; done" would grant every rm there will ever be.
	Generalizable bool
}

// Pattern is how a rule names this scope: "curl *" for an ordinary command, and
// the command as written for one that cannot be generalized.
func (s Scope) Pattern() string {
	if !s.Generalizable {
		return s.Literal
	}
	return s.Program + " *"
}

// String renders the scope for a person to read.
func (s Scope) String() string { return s.Literal }

// Parse finds every simple command in a command line, including the ones inside
// pipelines, chains, subshells, command substitutions, loops and conditionals.
func Parse(command string) ([]Scope, error) {
	f, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return nil, err
	}

	// A command inside a loop or a conditional cannot be generalized, so first
	// note where those constructs begin and end.
	type span struct{ start, end uint }
	var compounds []span
	syntax.Walk(f, func(node syntax.Node) bool {
		switch node.(type) {
		case *syntax.IfClause, *syntax.ForClause, *syntax.WhileClause,
			*syntax.CaseClause, *syntax.FuncDecl:
			compounds = append(compounds, span{node.Pos().Offset(), node.End().Offset()})
		}
		return true
	})

	var scopes []Scope
	syntax.Walk(f, func(node syntax.Node) bool {
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			return true
		}
		generalizable := true
		at := call.Pos().Offset()
		for _, c := range compounds {
			if at >= c.start && at < c.end {
				generalizable = false
				break
			}
		}
		if s, ok := scopeFromCall(call, generalizable); ok {
			scopes = append(scopes, s)
		}
		return true
	})
	return scopes, nil
}

func scopeFromCall(call *syntax.CallExpr, generalizable bool) (Scope, bool) {
	if len(call.Args) == 0 {
		// A bare assignment such as FOO=bar with no command after it.
		return Scope{}, false
	}

	words := make([]string, 0, len(call.Args))
	for _, arg := range call.Args {
		words = append(words, wordText(arg))
	}

	// An environment prefix (FOO=bar cmd) is parsed into Assigns rather than
	// Args, so the first word here really is the program.
	program := words[0]
	if base := filepath.Base(program); base != "" && base != "." {
		program = base
	}

	s := Scope{
		Program:       program,
		Args:          words[1:],
		Literal:       strings.Join(words, " "),
		Generalizable: generalizable,
	}
	for _, arg := range call.Args[1:] {
		if p, ok := pathArgument(arg); ok {
			s.Paths = append(s.Paths, p)
		}
	}
	return s, true
}

// wordText renders one word as it was written, which is what a person reading a
// grant prompt needs to see.
func wordText(w *syntax.Word) string {
	var sb strings.Builder
	if err := syntax.NewPrinter().Print(&sb, w); err != nil {
		return literalOf(w)
	}
	return sb.String()
}

// literalOf returns the word with its quoting removed.
func literalOf(w *syntax.Word) string {
	var sb strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			sb.WriteString(p.Value)
		case *syntax.SglQuoted:
			sb.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, inner := range p.Parts {
				if lit, ok := inner.(*syntax.Lit); ok {
					sb.WriteString(lit.Value)
				}
			}
		}
	}
	return sb.String()
}

// isLiteral reports whether the whole word is known now — no variables, no
// command substitution, no arithmetic. Only a literal word can be judged as a
// path, because anything else is not decided until the shell runs.
func isLiteral(w *syntax.Word) bool {
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit, *syntax.SglQuoted:
		case *syntax.DblQuoted:
			for _, inner := range p.Parts {
				if _, ok := inner.(*syntax.Lit); !ok {
					return false
				}
			}
		default:
			_ = p
			return false
		}
	}
	return true
}

// pathArgument decides whether an argument names a file or directory. Taking it
// from the parsed argument rather than from a regular expression over the whole
// line is what stops a flag, a grep pattern or a word inside a quoted string
// being mistaken for a path.
func pathArgument(w *syntax.Word) (string, bool) {
	if !isLiteral(w) {
		return "", false
	}
	value := literalOf(w)
	if value == "" || strings.HasPrefix(value, "-") {
		return "", false
	}
	// A URL contains slashes but is not a path on this machine.
	if i := strings.Index(value, "://"); i > 0 && !strings.ContainsAny(value[:i], "/.") {
		return "", false
	}
	switch {
	case value == "/" || value == "~":
		return value, true
	case strings.HasPrefix(value, "/"), strings.HasPrefix(value, "./"),
		strings.HasPrefix(value, "../"), strings.HasPrefix(value, "~/"),
		strings.Contains(value, "/"):
		return value, true
	}
	return "", false
}
