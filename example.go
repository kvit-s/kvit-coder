// Package kvitcoder holds files from the repository root that the programs
// need at run time, so there is one copy of each.
package kvitcoder

import _ "embed"

// ExampleConfig is config.example.yaml. kvit-coder-ui writes it to
// ~/.kvit-coder/config.yaml when it finds no configuration at all; the
// installers copy the same file from the release archive.
//
//go:embed config.example.yaml
var ExampleConfig []byte
