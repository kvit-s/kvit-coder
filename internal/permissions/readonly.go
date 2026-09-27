package permissions

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// ReadOnly reports whether running command can only read: every simple
// command in it is a program on the read-only list below, used without the
// flags that make it write, and no redirection sends output to a file.
//
// It answers "does this turn owe a check of what it changed", so being wrong
// in one direction is much worse than in the other. Calling a command
// read-only when it wrote something would excuse a turn that changed files
// from saying how it checked them, while calling a read-only command a change
// costs one not_run check on the report. Anything that cannot be settled from
// the text therefore counts as a change: a parse error, a program not on the
// list, a variable in the program's place, a script run by an interpreter.
func ReadOnly(command string) bool {
	f, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return false
	}
	readOnly := true
	sawCommand := false
	syntax.Walk(f, func(node syntax.Node) bool {
		if !readOnly {
			return false
		}
		switch n := node.(type) {
		case *syntax.Redirect:
			if writesFile(n) {
				readOnly = false
			}
		case *syntax.CallExpr:
			if len(n.Args) == 0 {
				// A bare assignment such as FOO=bar changes only the shell.
				return true
			}
			sawCommand = true
			if !isLiteral(n.Args[0]) {
				// The program is not known until the shell runs.
				readOnly = false
				return false
			}
			words := make([]string, 0, len(n.Args))
			literal := true
			for _, w := range n.Args {
				literal = literal && isLiteral(w)
				words = append(words, literalOf(w))
			}
			if !readOnlyCall(words, literal) {
				readOnly = false
			}
		}
		return true
	})
	return readOnly && sawCommand
}

// writesFile reports whether a redirection sends output anywhere but
// /dev/null or another file descriptor.
func writesFile(r *syntax.Redirect) bool {
	target := ""
	if r.Word != nil {
		target = literalOf(r.Word)
	}
	switch r.Op {
	case syntax.RdrOut, syntax.AppOut, syntax.ClbOut, syntax.RdrAll, syntax.AppAll, syntax.RdrInOut:
		return target != "/dev/null"
	case syntax.DplOut:
		// 2>&1 and >&- move or close a descriptor; >&file writes a file.
		return target != "-" && strings.Trim(target, "0123456789") != ""
	}
	return false
}

// readOnlyPrograms only read, whatever arguments they are given.
var readOnlyPrograms = map[string]bool{
	"[": true, "basename": true, "cat": true, "cd": true, "cmp": true, "column": true,
	"cut": true, "date": true, "df": true, "diff": true, "dirname": true, "du": true,
	"echo": true, "egrep": true, "false": true, "fgrep": true, "file": true, "grep": true,
	"head": true, "hostname": true, "id": true, "jq": true, "ls": true, "md5sum": true,
	"nl": true, "pgrep": true, "printenv": true, "printf": true, "ps": true, "pwd": true,
	"readlink": true, "realpath": true, "rg": true, "sha1sum": true, "sha256sum": true,
	"sha512sum": true, "stat": true, "tail": true, "test": true, "tr": true, "tree": true,
	"true": true, "uname": true, "uniq": true, "wc": true, "which": true, "whoami": true,
}

// readOnlyCall judges one simple command, given as its words with quoting
// removed. literal says every word was plain text; when one was not, the
// flags that decide whether a program writes cannot be seen, so only the
// programs that never write pass.
func readOnlyCall(words []string, literal bool) bool {
	program, args := words[0], words[1:]
	if i := strings.LastIndex(program, "/"); i >= 0 {
		program = program[i+1:]
	}
	if readOnlyPrograms[program] {
		return true
	}
	if !literal {
		return false
	}
	switch program {
	case "find":
		for _, a := range args {
			switch a {
			case "-delete", "-exec", "-execdir", "-ok", "-okdir", "-fprint", "-fprint0", "-fprintf", "-fls":
				return false
			}
		}
		return true
	case "sed":
		// -i edits in place, including inside a cluster such as -ni.
		return !anyShortFlag(args, 'i') && !anyArgPrefix(args, "--in-place")
	case "sort":
		return !anyShortFlag(args, 'o') && !anyArgPrefix(args, "--output")
	case "gofmt":
		// Go's flag package does not cluster, so -w only ever appears alone.
		return !anyArgPrefix(args, "-w")
	case "go":
		return readOnlyGo(args)
	case "git":
		return readOnlyGit(args)
	}
	return false
}

// readOnlyGo allows the go subcommands that report on the module without
// building anything into the workspace.
func readOnlyGo(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "doc", "list", "version", "vet":
		return true
	case "env":
		return !anyArgPrefix(args[1:], "-w") && !anyArgPrefix(args[1:], "-u")
	}
	return false
}

// readOnlyGit allows the git subcommands that only inspect the repository,
// and the listing forms of the few that can also change it.
func readOnlyGit(args []string) bool {
	// Global options come before the subcommand; -C and -c take a value.
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		if args[0] == "-C" || args[0] == "-c" {
			if len(args) < 2 {
				return false
			}
			args = args[1:]
		}
		args = args[1:]
	}
	if len(args) == 0 {
		return false
	}
	sub, rest := args[0], args[1:]
	if anyArgPrefix(rest, "--output") {
		return false
	}
	switch sub {
	case "blame", "cat-file", "check-ignore", "describe", "diff", "grep", "log",
		"ls-files", "ls-tree", "merge-base", "rev-list", "rev-parse", "shortlog",
		"show", "show-ref", "status":
		return true
	case "branch", "tag":
		for _, a := range rest {
			switch a {
			case "-a", "-r", "-v", "-vv", "-l", "--list", "--all", "--remotes", "--verbose", "--show-current":
			default:
				return false
			}
		}
		return true
	case "remote":
		return len(rest) == 0 || rest[0] == "-v" || rest[0] == "get-url" || rest[0] == "show"
	case "stash":
		return len(rest) > 0 && (rest[0] == "list" || rest[0] == "show")
	case "config":
		for _, a := range rest {
			switch a {
			case "--get", "--get-all", "--get-regexp", "--list", "-l":
				return true
			}
		}
		return false
	}
	return false
}

// anyShortFlag reports whether any argument is a cluster of single-letter
// flags that includes letter, such as -ni for 'i'.
func anyShortFlag(args []string, letter byte) bool {
	for _, a := range args {
		if len(a) > 1 && a[0] == '-' && a[1] != '-' && strings.IndexByte(a[1:], letter) >= 0 {
			return true
		}
	}
	return false
}

func anyArgPrefix(args []string, prefix string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			return true
		}
	}
	return false
}
