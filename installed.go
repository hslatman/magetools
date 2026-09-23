package magetools

import (
	"slices"
	"strings"
)

// Tool describes one installed tool, as recorded in its sidecar modfile.
type Tool struct {
	// Package is the tool package path, e.g.
	// github.com/golangci/golangci-lint/v2/cmd/golangci-lint.
	Package string
	// Version is the version pinned in .magetools/<slug>/go.mod. It is empty
	// when the modfile pins a tool with no matching require entry, which is how
	// callers tell an unversioned tool apart from one that is not installed.
	Version string
	// Binary is the name the tool is invoked as, e.g. golangci-lint.
	Binary string
	// Modfile is the path to the sidecar modfile, relative to the module root,
	// e.g. .magetools/github-com-.../go.mod. Callers that want to run the tool
	// themselves — with their own context and output capture — pass it to
	// "go tool -modfile <Modfile> <Binary>".
	Modfile string
}

// Installed returns every installed tool, sorted by package path so callers get
// a deterministic order. Directories under the tool directory that do not hold a
// valid tool modfile are skipped rather than reported, matching List: they are
// leftovers, not failures. A missing tool directory yields no tools and no error.
//
// Installed inspects the tool directory relative to the current working
// directory, and the returned Modfile paths are relative too; callers scanning
// several repositories must chdir into each. That working directory must also
// be inside a Go module, because the sidecar modfiles are read with "go mod
// edit -modfile", which the go command rejects outside one.
//
// NOTE: this signature does not match a Mage target, so Mage skips it with a
// notice under -debug. That is intended; it is an inspection API for other Go
// code, not a target.
func Installed() ([]Tool, error) {
	slugs, err := installedSlugs()
	if err != nil {
		return nil, err
	}

	tools := make([]Tool, 0, len(slugs))
	for _, slug := range slugs {
		r, err := newRunnerFromSlug(slug)
		if err != nil {
			return nil, err
		}

		info, err := r.toolInfo()
		if err != nil {
			// Skip directories that aren't valid tool modules.
			continue
		}

		tools = append(tools, Tool{
			Package: info.Package,
			Version: info.Version,
			Binary:  computeBinaryName(info.Package),
			Modfile: r.modFile,
		})
	}

	// Package is not a guaranteed-unique key: a hand-edited or migrated tool
	// directory can hold two modfiles pinning the same package. Tiebreak on the
	// modfile path so the order is total, and therefore deterministic.
	slices.SortFunc(tools, func(a, b Tool) int {
		if c := strings.Compare(a.Package, b.Package); c != 0 {
			return c
		}
		return strings.Compare(a.Modfile, b.Modfile)
	})

	return tools, nil
}
