package main

import "strings"

// imagePathList is a repeatable -image flag: -image a.png -image b.png.
type imagePathList []string

func (l *imagePathList) String() string { return strings.Join(*l, ", ") }

func (l *imagePathList) Set(v string) error {
	*l = append(*l, v)
	return nil
}
