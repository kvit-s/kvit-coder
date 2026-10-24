package compaction

import "strings"

// turnInstruction asks the model to compact the turn in progress. The request
// it ends is an earlier request of the same turn, so the messages above it are
// exactly the part of the turn to compact: the word-for-word tail that follows
// in the conversation is not in it.
const turnInstruction = `The conversation is being shortened to fit your context window. Write a compacted version of the work done in the current turn, up to the last message above, for yourself to continue from. The current turn began with the user's most recent request; requests before it that ended without an answer are part of it too. The user's own words are kept separately and need not be restated, and the messages that come after this point will be kept word for word.

Use terse Markdown bullets under these headings, keeping exact file paths, symbols, commands and error messages:

## Learned
What the work found out that the rest of the request needs: what the files and outputs you read actually say, how the code works, the facts you would otherwise have to read again. Record the content, not just that you read it.
## Changed
## Tried and failed
## Decisions
## Remaining

Keep anything from earlier in the conversation that the rest of this turn depends on, including text compacted earlier in this turn. A tool output shown as "[output removed to fit the context window]" was removed to fit; do not guess what it said. Do not call tools, and do not mention that this is a summary.`

// olderInstruction asks the model to summarize the oldest turns, which reach
// it as turn summaries: each turn's words and the answer it ended with.
const olderInstruction = `The turns above will be replaced by your summary of them, to keep the conversation inside your context window. Summarize them for yourself in terse Markdown bullets under these headings, keeping exact file paths, symbols, commands and error messages:

## Goals
## Constraints and preferences
## Done
## Decisions
## Relevant files

Do not call tools, and do not mention that this is a summary.`

// withFocus appends the configured instructions and a focus given with
// :compact, when there are any.
func withFocus(instruction, configured, focus string) string {
	var extra []string
	if s := strings.TrimSpace(configured); s != "" {
		extra = append(extra, s)
	}
	if s := strings.TrimSpace(focus); s != "" {
		extra = append(extra, s)
	}
	if len(extra) == 0 {
		return instruction
	}
	return instruction + "\n\nAdditional focus: " + strings.Join(extra, "\n")
}
