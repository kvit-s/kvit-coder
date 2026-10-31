package main

import "strings"

// pathList is a repeatable path flag: -image a.png -image b.png, and the same
// for -paste.
type pathList []string

func (l *pathList) String() string { return strings.Join(*l, ", ") }

func (l *pathList) Set(v string) error {
	*l = append(*l, v)
	return nil
}
