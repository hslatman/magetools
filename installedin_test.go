package magetools

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// module creates a Go module with one pinned tool, and returns its directory.
func module(t *testing.T, toolPackage, version string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module example.com/probe\n\ngo 1.25.0\n")

	slug := slugify(toolPackage)
	if err := os.MkdirAll(filepath.Join(dir, toolDir, slug), 0o755); err != nil {
		t.Fatalf("creating tool dir: %v", err)
	}
	// The version stays in v0/v1: a v2+ version on a module path without a
	// matching /v2 suffix is a malformed requirement, and the go command
	// rejects the whole modfile rather than the one line.
	write(t, filepath.Join(dir, toolDir, slug, "go.mod"),
		"module tool\n\ngo 1.25.0\n\ntool "+toolPackage+"\n\nrequire "+toolPackage+" "+version+" // indirect\n")
	return dir
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// InstalledIn reads a repository the caller is not standing in. Without it the
// only way to inspect another repository is to chdir into it, which is
// process-global and therefore cannot be done concurrently.
func TestInstalledInReadsAnotherDirectory(t *testing.T) {
	dir := module(t, "example.com/x/cmd/lint", "v1.2.3")

	got, err := InstalledIn(dir)
	if err != nil {
		t.Fatalf("InstalledIn(): %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %d tools, want 1: %+v", len(got), got)
	}
	if got[0].Package != "example.com/x/cmd/lint" || got[0].Version != "v1.2.3" {
		t.Errorf("tools[0] = %+v", got[0])
	}
	if got[0].Binary != "lint" {
		t.Errorf("Binary = %q, want %q", got[0].Binary, "lint")
	}
}

// The point of taking a directory is that the working directory stays put.
func TestInstalledInDoesNotChangeTheWorkingDirectory(t *testing.T) {
	before, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := module(t, "example.com/x/cmd/lint", "v1.2.3")

	if _, err := InstalledIn(dir); err != nil {
		t.Fatalf("InstalledIn(): %v", err)
	}

	after, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if after != before {
		t.Errorf("working directory moved to %q, want %q", after, before)
	}
}

// Two repositories can be read at once. This is what the directory argument
// buys, and it is worth asserting rather than assuming: run under -race.
func TestInstalledInIsSafeForConcurrentUse(t *testing.T) {
	first := module(t, "example.com/a/cmd/one", "v1.0.0")
	second := module(t, "example.com/b/cmd/two", "v1.9.0")

	var wg sync.WaitGroup
	results := make([][]Tool, 2)
	errs := make([]error, 2)
	for i, dir := range []string{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = InstalledIn(dir)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("InstalledIn() [%d]: %v", i, err)
		}
	}
	if len(results[0]) != 1 || results[0][0].Binary != "one" {
		t.Errorf("first = %+v, want the tool from the first repository", results[0])
	}
	if len(results[1]) != 1 || results[1][0].Binary != "two" {
		t.Errorf("second = %+v, want the tool from the second repository", results[1])
	}
}

// Outside a Go module the answer is an error, not an empty list. Reporting
// "no tools installed" for a directory that was never a repository turns a
// misconfiguration into a drift table that lies.
func TestInstalledOutsideAModuleIsAnError(t *testing.T) {
	dir := t.TempDir() // no go.mod anywhere above it

	_, err := InstalledIn(dir)

	if err == nil {
		t.Fatal("InstalledIn(non-module) = nil error, want an error")
	}
	if !errors.Is(err, ErrNotAModule) {
		t.Errorf("errors.Is(err, ErrNotAModule) = false for %v", err)
	}
}

// Installed keeps working, reading the current directory.
func TestInstalledUsesTheWorkingDirectory(t *testing.T) {
	dir := module(t, "example.com/x/cmd/lint", "v1.2.3")
	t.Chdir(dir)

	got, err := Installed()
	if err != nil {
		t.Fatalf("Installed(): %v", err)
	}
	if len(got) != 1 || got[0].Binary != "lint" {
		t.Errorf("Installed() = %+v", got)
	}
}

// Update and List hide the same failure behind the same skip loop, so they get
// the same honest answer.
func TestUpdateOutsideAModuleIsAnError(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := Update(); !errors.Is(err, ErrNotAModule) {
		t.Errorf("Update() outside a module = %v, want ErrNotAModule", err)
	}
}

func TestListOutsideAModuleIsAnError(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := List(); !errors.Is(err, ErrNotAModule) {
		t.Errorf("List() outside a module = %v, want ErrNotAModule", err)
	}
}
