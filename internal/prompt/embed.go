package prompt

import (
	"embed"
)

// embeddedPrompts contains all template files from the prompts directory.
// This allows templates to be compiled into the binary while still being
// overridable from the filesystem.
//
//go:embed prompts/*
var embeddedPrompts embed.FS
