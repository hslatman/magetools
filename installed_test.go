package magetools

import (
	"path/filepath"
	"testing"
)

func TestInstalled(t *testing.T) {
	setToolDir(t, filepath.Join("testdata", "installed"))

	got, err := Installed()
	if err != nil {
		t.Fatalf("Installed(): %v", err)
	}

	want := []Tool{
		{
			Package: "github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
			Version: "v2.12.2",
			Binary:  "golangci-lint",
			Modfile: filepath.Join("testdata", "installed", "github-com-golangci-golangci-lint-v2-cmd-golangci-lint", "go.mod"),
		},
		{
			Package: "golang.org/x/vuln/cmd/govulncheck",
			Version: "v1.1.4",
			Binary:  "govulncheck",
			Modfile: filepath.Join("testdata", "installed", "golang-org-x-vuln-cmd-govulncheck", "go.mod"),
		},
		// The next two exist to make sorting observable: they are read from disk
		// in zz-a, zz-b order, but must be reported in the opposite order,
		// because slugify maps both "." and "/" to "-" and "-" sorts before "."
		// in a package path. Without the sort in Installed, these two swap and
		// this test fails — which is the point of them.
		{
			Package: "zz-b.example.com/tool",
			Version: "v1.1.0",
			Binary:  "tool",
			Modfile: filepath.Join("testdata", "installed", "zz-b-example-com-tool", "go.mod"),
		},
		{
			Package: "zz.a.example.com/tool",
			Version: "v1.0.0",
			Binary:  "tool",
			Modfile: filepath.Join("testdata", "installed", "zz-a-example-com-tool", "go.mod"),
		},
	}

	if len(got) != len(want) {
		t.Fatalf("Installed() returned %d tools, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Installed()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// Installed must sort by package path so that consumers' output is diffable
// across runs.
//
// The zz-a/zz-b fixtures are read from disk in the inverse of their sorted
// package order, so this test fails if the sort in Installed is removed.
// Do not delete them: without a pair whose slug order and package order
// disagree, this test would pass no matter what Installed did.
func TestInstalledIsSorted(t *testing.T) {
	setToolDir(t, filepath.Join("testdata", "installed"))

	got, err := Installed()
	if err != nil {
		t.Fatalf("Installed(): %v", err)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Package >= got[i].Package {
			t.Errorf("Installed() not sorted by package: %q before %q", got[i-1].Package, got[i].Package)
		}
	}
}

func TestInstalledNoToolDirectory(t *testing.T) {
	setToolDir(t, filepath.Join("testdata", "does-not-exist"))

	got, err := Installed()
	if err != nil {
		t.Fatalf("Installed() with missing tool dir: %v, want nil error", err)
	}
	if len(got) != 0 {
		t.Errorf("Installed() with missing tool dir = %+v, want empty", got)
	}
}
