package tools

import "fmt"

// The short forms of the tool documentation, used under the strong agent
// profile. The schema already says what each tool takes; what it cannot say is
// where the tool will surprise you — what gets truncated, what has to match
// byte for byte, what does not persist between calls. That is all these say.

// ShortPromptSection gives Read's limits, which the schema cannot express.
func (t *ReadFileTool) ShortPromptSection() string {
	return fmt.Sprintf(`### Read

Truncates at %d lines or %dKB, whichever comes first; use start and limit to
read a large file in pieces. A negative start counts from the end. Files
only: a directory path is refused, use Glob to list it.`,
		t.maxLines, t.maxBytes/1024)
}

// ShortPromptSection says what Glob returns and where it stops.
func (t *GlobTool) ShortPromptSection() string {
	max := t.config.Tools.Glob.MaxResults
	if max == 0 {
		max = DefaultGlobMaxResults
	}
	return fmt.Sprintf(`### Glob

Finds files by pattern ('**/*.go'), scoped to path (default: workspace).
No pattern lists the directory at path. Sorted, capped at %d paths with a
truncation flag past it; excluded dirs (.git, node_modules) never walked.`, max)
}

// ShortPromptSection gives the searchreplace Edit's one hard rule.
func (t *SearchReplaceEditTool) ShortPromptSection() string {
	base := `### Edit

'search' must match the file byte for byte, whitespace included, and must match
exactly once — include enough surrounding lines to make it unique, and the tool
will tell you when there is more than one match. Read the file first: editing
from memory is where this goes wrong.`
	if t.Config.Tools.Edit.PreviewMode {
		base += "\n\nThe edit is staged, not applied: call Edit.confirm to apply it or Edit.cancel to drop it."
	}
	return base
}

// ShortPromptSection gives the line-mode Edit's one hard rule.
func (t *UnifiedEditTool) ShortPromptSection() string {
	base := `### Edit

start_line and end_line are 1-based and inclusive, and replace exactly those
lines. Omit end_line to insert without replacing; new_text of "" deletes the
lines. Line numbers go stale the moment anything else edits the file, so Read
it immediately before editing.`
	if t.Config.Tools.Edit.PreviewMode {
		base += "\n\nThe edit is staged, not applied: call Edit.confirm to apply it or Edit.cancel to drop it."
	}
	return base
}

// ShortPromptSection says what Search does when there is too much to show.
func (t *SearchTool) ShortPromptSection() string {
	maxSnippet := t.config.Tools.Search.MaxSnippetResults
	if maxSnippet == 0 {
		maxSnippet = 20
	}
	maxCompact := t.config.Tools.Search.MaxCompactResults
	if maxCompact == 0 {
		maxCompact = 100
	}
	return fmt.Sprintf(`### Search

Results degrade as they grow: full snippets up to %d matches, then file:line
only up to %d, then the whole result is written to a temp file whose path you
are given. Narrow with path and file_pattern rather than reading everything.`,
		maxSnippet, maxCompact)
}

// ShortPromptSection says the two things about Shell that are not in its schema.
func (t *ShellTool) ShortPromptSection() string {
	return fmt.Sprintf(`### Shell

Each call is a fresh shell in %s: a cd, an export or a shell variable does not
survive to the next call, so chain what has to happen together in one command
or use Shell.advanced with working_dir. Output beyond a few hundred lines is
truncated and the whole of it written to a temp file whose path you are given;
read that rather than running the command again.`, t.advanced.workspaceRoot)
}

// ShortPromptSection folds Shell.advanced into a line, since it differs from
// Shell only in taking two more arguments the schema already describes.
func (t *ShellAdvancedTool) ShortPromptSection() string {
	return fmt.Sprintf(`### Shell.advanced

Shell with a working_dir and a timeout. Default %ds, maximum %ds.`,
		int(t.timeout.Seconds()), int(t.maxTimeout().Seconds()))
}

// ShortPromptSection says the one thing about Write that is not in its schema.
func (t *WriteFileTool) ShortPromptSection() string {
	base := `### Write

Writes the whole file. Overwriting an existing file replaces it entirely, so
use Edit for a change to a file that already exists.`
	if t.config.Tools.Edit.PreviewMode {
		base += "\n\nOverwriting is staged: call Write.confirm to apply it or Write.cancel to drop it."
	}
	return base
}

// ShortPromptSection keeps only Batch's limits; when to reach for it is said
// once, in the prompt's account of how the session works.
func (t *BatchTool) ShortPromptSection() string {
	return fmt.Sprintf(`### Batch

Takes a list of {tool, args}, at most %d, with no Batch inside a Batch.
Read-only calls run at the same time and the rest in order afterwards. One
call failing does not stop the others: each comes back with its own result or
its own error, against the position it was given in.`, maxBatchCalls)
}

// ShortPromptSection keeps only what happens when a question is put; when to
// ask is said once, in the prompt's account of how the session works.
func (t *QuestionTool) ShortPromptSection() string {
	return `### Question

Blocks until it is answered, and the answer comes back as the tool's result.
Take every question you have in one call. If no one answers you are told so,
and are expected to carry on and say what you assumed.`
}

// ShortPromptSection says the one thing that separates Shell.start from Shell.
func (t *ShellStartTool) ShortPromptSection() string {
	return `### Shell.start

The command outlives the turn's iterations (ephemeral: stopped at turn end;
persistent=true survives, red). Use Shell
for anything that finishes on its own. Read what it has printed with
Shell.output, wait for it with Observe.wait, stop it with Shell.kill.`
}

// ShortPromptSection says what Observe.wait returns and why.
func (t *ObserveWaitTool) ShortPromptSection() string {
	return `### Observe.wait

Blocks until the process ends, prints something matching 'until', or max_wait
seconds pass, and says which of those it returned for. It also returns as soon
as someone types, so you are not left waiting for something that no longer
matters.`
}

// ShortPromptSection says what Web.search cannot express in its schema: what a
// result is for, and that the two refusals mean different things.
func (t *WebSearchTool) ShortPromptSection() string {
	return `### Web.search

A result is a title, a URL and a couple of sentences -- enough to choose a page
with, not the page itself. Read what you choose with Web.fetch.

One request per second and a fixed monthly allowance, shared with other programs
using the same key. A per-second collision is waited out and retried here, so it
never reaches you. An exhausted month does, and it will not clear for weeks:
answer from what you know rather than trying again. Each result says how many
searches are left in the month.`
}

// ShortPromptSection says what Web.fetch cannot express in its schema: the two
// result shapes, and the one failure that looks like success.
func (t *WebFetchTool) ShortPromptSection() string {
	return `### Web.fetch

The page is converted to markdown and written to a file. A short page comes back
whole in content; a long one comes back as head plus an outline of its headings
with line numbers, and you Read the file at those offsets for the rest.

Static HTML only. A page that builds itself with JavaScript, or answers with a
bot check, comes back nearly empty with a warning saying so -- that is not a
page with nothing on it. Prefer an address that needs no rendering where one
exists: raw.githubusercontent.com over the GitHub file view, pkg.go.dev over a
rendered doc site, an llms.txt or .txt variant where published.`
}
