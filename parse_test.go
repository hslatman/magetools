package magetools

import "testing"

// setToolDir points the package at a different tool directory for the duration
// of a test. toolDir is a package variable read once at init from
// MAGETOOLS_DIRECTORY, so t.Setenv has no effect on it; tests using this helper
// must not call t.Parallel.
func setToolDir(t *testing.T, dir string) {
	t.Helper()
	old := toolDir
	toolDir = dir
	t.Cleanup(func() { toolDir = old })
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "github.com/magefile/mage", "github-com-magefile-mage"},
		{"drops version suffix", "github.com/go-task/task/v3/cmd/task@latest", "github-com-go-task-task-v3-cmd-task"},
		{"drops pinned version", "golang.org/x/vuln/cmd/govulncheck@v1.1.4", "golang-org-x-vuln-cmd-govulncheck"},
		{"uppercase is lowered", "github.com/Foo/Bar", "github-com-foo-bar"},
		{"runs of unsafe characters collapse", "github.com/foo--bar/_baz", "github-com-foo-bar-baz"},
		{"leading and trailing separators trimmed", "/github.com/foo/", "github-com-foo"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := slugify(tt.in); got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestComputeBinaryName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"cmd subdirectory", "github.com/go-task/task/v3/cmd/task@latest", "task"},
		{"module root", "github.com/magefile/mage", "mage"},
		{"major version suffix is skipped", "github.com/goreleaser/goreleaser/v2", "goreleaser"},
		{"major version inside path is not skipped", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint", "golangci-lint"},
		{"version stripped before basename", "golang.org/x/vuln/cmd/govulncheck@v1.1.4", "govulncheck"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := computeBinaryName(tt.in); got != tt.want {
				t.Errorf("computeBinaryName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeToolchainVersion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"language version gains a patch", "1.24", "1.24.0"},
		{"full version unchanged", "1.25.0", "1.25.0"},
		{"patch version unchanged", "1.24.3", "1.24.3"},
		{"empty unchanged", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeToolchainVersion(tt.in); got != tt.want {
				t.Errorf("normalizeToolchainVersion(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
