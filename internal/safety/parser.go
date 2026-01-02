package safety

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// ParsedCommand represents a parsed shell command
type ParsedCommand struct {
	Binary     string            // The executable (e.g., "git", "rm")
	Subcommand string            // First positional arg if applicable (e.g., "push", "reset")
	Flags      map[string]bool   // Present flags (e.g., "-rf" -> {"r": true, "f": true})
	FlagArgs   map[string]string // Flags with values (e.g., "-c" -> "inner command")
	Args       []string          // Positional arguments
	Raw        string            // Original command string
	Pipes      []*ParsedCommand  // Commands after | (for pipeline analysis)
}

// ParseCommand parses a shell command string into structured components
func ParseCommand(cmd string) (*ParsedCommand, error) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil, fmt.Errorf("empty command")
	}

	// First, split by pipes (outside of quotes)
	segments, err := splitPipeline(cmd)
	if err != nil {
		return nil, err
	}

	// Parse the first command
	result, err := parseSegment(segments[0])
	if err != nil {
		return nil, err
	}
	result.Raw = cmd

	// Parse subsequent pipe segments
	for i := 1; i < len(segments); i++ {
		pipeCmd, err := parseSegment(segments[i])
		if err != nil {
			return nil, fmt.Errorf("pipe segment %d: %w", i, err)
		}
		result.Pipes = append(result.Pipes, pipeCmd)
	}

	return result, nil
}

// splitPipeline splits a command by pipes, respecting quotes
func splitPipeline(cmd string) ([]string, error) {
	var segments []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]

		if escaped {
			current.WriteByte(c)
			escaped = false
			continue
		}

		if c == '\\' {
			current.WriteByte(c)
			escaped = true
			continue
		}

		if c == '\'' && !inDouble {
			inSingle = !inSingle
			current.WriteByte(c)
			continue
		}

		if c == '"' && !inSingle {
			inDouble = !inDouble
			current.WriteByte(c)
			continue
		}

		if c == '|' && !inSingle && !inDouble {
			seg := strings.TrimSpace(current.String())
			if seg != "" {
				segments = append(segments, seg)
			}
			current.Reset()
			continue
		}

		current.WriteByte(c)
	}

	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quote")
	}

	seg := strings.TrimSpace(current.String())
	if seg != "" {
		segments = append(segments, seg)
	}

	if len(segments) == 0 {
		return nil, fmt.Errorf("no command segments found")
	}

	return segments, nil
}

// parseSegment parses a single command segment (without pipes)
func parseSegment(segment string) (*ParsedCommand, error) {
	tokens, err := tokenize(segment)
	if err != nil {
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, fmt.Errorf("no tokens found")
	}

	result := &ParsedCommand{
		Flags:    make(map[string]bool),
		FlagArgs: make(map[string]string),
		Args:     []string{},
		Raw:      segment,
	}

	// First token is the binary
	result.Binary = filepath.Base(tokens[0]) // Handle /usr/bin/git -> git
	if result.Binary == "" {
		result.Binary = tokens[0]
	}

	// Process remaining tokens
	i := 1
	for i < len(tokens) {
		token := tokens[i]

		// Handle "--" (argument terminator) - treat as positional arg, not flag
		if token == "--" {
			result.Args = append(result.Args, token)
			i++
			continue
		}

		if strings.HasPrefix(token, "--") {
			// Long flag
			flag, value := parseLongFlag(token)
			if value != "" {
				result.FlagArgs[flag] = value
			} else {
				result.Flags[flag] = true
				// Check if next token is the value (for --flag value syntax)
				if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
					// Some flags take values, but we can't know which ones
					// So we just mark the flag as present
				}
			}
		} else if strings.HasPrefix(token, "-") && len(token) > 1 {
			// Short flag(s)
			flagStr := token[1:]

			// Check for -flag=value pattern
			if eqIdx := strings.Index(flagStr, "="); eqIdx != -1 {
				flag := flagStr[:eqIdx]
				value := flagStr[eqIdx+1:]
				result.FlagArgs[flag] = value
			} else {
				// Check if this is a flag with attached value (like -c'command')
				// or combined short flags (like -rf)
				expanded := expandShortFlags(flagStr)

				// Handle special case: flag that takes an argument (like -c)
				for j, flag := range expanded {
					result.Flags[flag] = true

					// If this is the last expanded flag and next token exists,
					// check if it could be a flag argument
					if j == len(expanded)-1 && i+1 < len(tokens) {
						nextToken := tokens[i+1]
						// For certain known flags, capture the argument
						if flag == "c" || flag == "e" || flag == "o" || flag == "I" {
							if !strings.HasPrefix(nextToken, "-") || flag == "c" || flag == "e" {
								result.FlagArgs[flag] = nextToken
								i++
							}
						}
					}
				}
			}
		} else {
			// Positional argument
			result.Args = append(result.Args, token)

			// First positional arg is the subcommand for certain commands
			if result.Subcommand == "" {
				result.Subcommand = token
			}
		}
		i++
	}

	return result, nil
}

// parseLongFlag parses a --flag or --flag=value
func parseLongFlag(token string) (flag, value string) {
	token = token[2:] // Remove --
	if eqIdx := strings.Index(token, "="); eqIdx != -1 {
		return token[:eqIdx], token[eqIdx+1:]
	}
	return token, ""
}

// expandShortFlags expands combined short flags like "-rf" to ["r", "f"]
func expandShortFlags(flags string) []string {
	var result []string
	for _, r := range flags {
		result = append(result, string(r))
	}
	return result
}

// tokenize splits a command string into tokens, respecting quotes
func tokenize(cmd string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	flushToken := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]

		if escaped {
			current.WriteByte(c)
			escaped = false
			continue
		}

		if c == '\\' && !inSingle {
			// In double quotes or unquoted, backslash escapes next char
			if inDouble || (!inSingle && !inDouble) {
				escaped = true
				// Don't include the backslash in the token for certain chars
				if i+1 < len(cmd) {
					next := cmd[i+1]
					if next == '"' || next == '\\' || next == '$' || next == '`' || next == '\n' {
						continue
					}
				}
			}
			current.WriteByte(c)
			continue
		}

		if c == '\'' && !inDouble {
			inSingle = !inSingle
			// Don't include the quote in the token
			continue
		}

		if c == '"' && !inSingle {
			inDouble = !inDouble
			// Don't include the quote in the token
			continue
		}

		if unicode.IsSpace(rune(c)) && !inSingle && !inDouble {
			flushToken()
			continue
		}

		current.WriteByte(c)
	}

	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quote")
	}

	flushToken()
	return tokens, nil
}

// HasFlag checks if a flag is present (supports both short and long forms)
func (p *ParsedCommand) HasFlag(flags ...string) bool {
	for _, flag := range flags {
		// Remove leading dashes for comparison
		flag = strings.TrimLeft(flag, "-")
		if p.Flags[flag] {
			return true
		}
		// Also check in FlagArgs (flags with values)
		if _, ok := p.FlagArgs[flag]; ok {
			return true
		}
	}
	return false
}

// HasAnyDestructiveRmFlags checks for recursive and force flags
func (p *ParsedCommand) HasAnyDestructiveRmFlags() (recursive, force bool) {
	recursive = p.HasFlag("r", "R", "recursive")
	force = p.HasFlag("f", "force")
	return
}

// GetFlagArg returns the argument for a flag, if any
func (p *ParsedCommand) GetFlagArg(flag string) (string, bool) {
	flag = strings.TrimLeft(flag, "-")
	val, ok := p.FlagArgs[flag]
	return val, ok
}

// HasDoubleDash returns true if "--" appears in the arguments
// (often used to separate flags from paths, like git checkout -- file)
func (p *ParsedCommand) HasDoubleDash() bool {
	for _, arg := range p.Args {
		if arg == "--" {
			return true
		}
	}
	return false
}

// ArgsAfterDoubleDash returns arguments after "--"
func (p *ParsedCommand) ArgsAfterDoubleDash() []string {
	for i, arg := range p.Args {
		if arg == "--" && i+1 < len(p.Args) {
			return p.Args[i+1:]
		}
	}
	return nil
}
