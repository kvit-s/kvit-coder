package model

import "log/slog"

// Log receives what the parsers report about parts they skipped. It is not in
// the upstream code, which logs to slog's default logger: in kvit-coder that
// would print over the terminal interface, and setting slog's default also
// redirects the standard log package. The office package swaps in a logger
// that collects warnings for the duration of one conversion.
var Log = slog.New(slog.DiscardHandler)
