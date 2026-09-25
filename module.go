package magetools

import (
	"errors"
	"fmt"
	"os"
)

// ErrNotAModule reports that a directory is not inside a Go module.
//
// It is a sentinel because the alternative is worse than an error: every
// function here that walks the tool directory skips entries it cannot parse,
// and outside a module the go command cannot parse any of them. Without this
// check the answer would be "no tools installed", which is indistinguishable
// from a repository that genuinely has none -- so a misconfigured caller would
// get a confident wrong answer instead of a problem to fix.
var ErrNotAModule = errors.New("not a Go module")

// requireModule reports whether dir is inside a Go module. An empty dir means
// the working directory.
//
// It asks the go command rather than looking for a go.mod, because a
// subdirectory of a module has no go.mod of its own and is still perfectly
// valid to work in. Outside a module the go command answers os.DevNull.
func requireModule(dir string) error {
	out, err := outputCmdIn(dir, "go", "env", "GOMOD")
	if err != nil {
		return fmt.Errorf("magetools: unable to tell whether %s is in a Go module: %w", describe(dir), err)
	}
	if out == "" || out == os.DevNull {
		return fmt.Errorf("magetools: %s is %w", describe(dir), ErrNotAModule)
	}
	return nil
}

// describe names a directory for an error message, so that the empty string
// reads as something a person recognises.
func describe(dir string) string {
	if dir == "" {
		return "the working directory"
	}
	return dir
}
