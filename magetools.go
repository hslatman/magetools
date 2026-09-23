// Package magetools installs and runs developer tools without adding them to
// the application's own module.
//
// Every tool gets its own sidecar modfile at .magetools/<slug>/go.mod, where
// <slug> is derived from the tool's package path. That modfile pins the tool
// with the Go 1.24+ "tool" directive and records the version of the module
// providing it, so the application's own go.mod and go.sum stay free of tool
// dependencies and a tool upgrade can never move the application's
// requirements. Tools are installed with "go get -modfile <modfile> -tool" and
// executed with "go tool -modfile <modfile> <binary>", which builds them from
// the versions pinned in that sidecar modfile.
//
// The exported functions double as Mage targets: Add installs a tool, Get
// reinstalls every pinned tool at its pinned version, List lists them, and
// Update is the only operation that moves a version. Installed exposes the same
// inventory to other Go code.
package magetools

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/magefile/mage/sh"
)

// TODO: describe relation with gomodtool
// TODO: add tests
// TODO: add examples / documentation

var toolDir = ".magetools"

func init() {
	if dir := os.Getenv("MAGETOOLS_DIRECTORY"); dir != "" {
		toolDir = dir
	}
}

// TODO: decide whether we want this to work; it's exposed as
// target otherwise, which is not what we want.
// func SetToolDir(dir string) {
// 	toolDir = dir
// }

var majorVersion = regexp.MustCompile(`^v\d+$`)

// slugUnsafe matches runs of characters that aren't allowed in a slug, so they
// can be collapsed into a single separator.
var slugUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

type runner struct {
	packageName string
	binaryName  string
	slug        string
	modFile     string
}

func (r *runner) get() error {
	if err := r.mkdirAll(); err != nil {
		return err
	}
	if !r.modfileExists() {
		if err := r.runCmd("go", "mod", "init", "-modfile", r.modFile, r.binaryName); err != nil {
			return err
		}
	}
	if r.packageName == "" {
		return fmt.Errorf(`magetools: no package name available for %s`, r.binaryName)
	}
	if err := r.runCmd("go", "get", "-modfile", r.modFile, "-tool", r.packageName); err != nil {
		return err
	}
	return nil
}

func (r *runner) run(args ...string) error {
	if !r.modfileExists() {
		return fmt.Errorf("magetools: no modfile for %s", r.binaryName)
	}
	args = append(
		[]string{"tool", "-modfile", r.modFile, r.binaryName},
		args...,
	)
	if err := r.runCmd("go", args...); err != nil {
		return err
	}
	return nil
}

func (r *runner) mkdirAll() error {
	dir := filepath.Dir(r.modFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf(`magetools: unable to create "%s" directory: %w`, dir, err)
	}
	return nil
}

func (r *runner) modfileExists() bool {
	_, err := os.Stat(r.modFile)
	return err == nil
}

// toolInfoResult describes the single tool pinned in a tool modfile.
type toolInfoResult struct {
	// Package is the tool package path from the "tool" directive, e.g.
	// github.com/golangci/golangci-lint/v2/cmd/golangci-lint.
	Package string
	// Module is the module providing that package: the longest required module
	// path that prefixes Package. Update needs it, because "go get -u" is
	// applied to a module.
	Module string
	// Version is the version Module is pinned at in the modfile.
	Version string
}

// toolInfo returns the tool pinned in the runner's modfile. It is the thin exec
// wrapper; all parsing lives in parseToolInfo so it can be tested offline.
func (r *runner) toolInfo() (toolInfoResult, error) {
	if !r.modfileExists() {
		return toolInfoResult{}, fmt.Errorf("magetools: no modfile for %s", r.slug)
	}
	content, err := outputCmd("go", "mod", "edit", "-modfile", r.modFile, "-json")
	if err != nil {
		return toolInfoResult{}, fmt.Errorf("magetools: unable to get tool from %s: %w", r.modFile, err)
	}
	info, err := parseToolInfo([]byte(content))
	if err != nil {
		return toolInfoResult{}, fmt.Errorf("magetools: %s: %w", r.modFile, err)
	}
	return info, nil
}

// parseToolInfo parses the output of "go mod edit -json" and resolves the tool
// package path to the module that provides it. The version is taken from the
// required module whose path is the longest prefix of the tool package path, so
// that reinstalling a tool restores the exact version rather than upgrading it.
func parseToolInfo(content []byte) (toolInfoResult, error) {
	var data struct {
		Tools []struct {
			Path string `json:"Path"`
		} `json:"Tool"`
		Require []struct {
			Path    string `json:"Path"`
			Version string `json:"Version"`
		} `json:"Require"`
	}
	if err := json.Unmarshal(content, &data); err != nil {
		return toolInfoResult{}, fmt.Errorf(`unable to parse output of "go mod edit -json": %w`, err)
	}

	if len(data.Tools) == 0 {
		return toolInfoResult{}, fmt.Errorf("no tool found")
	}

	// NOTE: currently assumes a single tool is present
	info := toolInfoResult{Package: data.Tools[0].Path}

	for _, req := range data.Require {
		if req.Path == info.Package || strings.HasPrefix(info.Package, req.Path+"/") {
			if len(req.Path) > len(info.Module) {
				info.Module = req.Path
				info.Version = req.Version
			}
		}
	}

	return info, nil
}

// TODO: add option to show/debug command that's going to run?
// runCmd runs a command, inheriting the caller's environment unchanged.
//
// It deliberately does not set GOTOOLCHAIN. Pinning it to the application's go
// directive made any tool whose own go.mod requires a newer Go impossible to
// install, and a "+auto" form instead makes the go command re-exec into a
// different toolchain, which "go get -modfile" does not survive. Choosing a
// toolchain policy is the caller's business; Go already defaults to "auto",
// and a caller who set "local" deliberately should keep it.
func (r *runner) runCmd(program string, args ...string) error {
	return sh.RunV(program, args...)
}

// TODO: add option to show/debug command that's going to run?
func outputCmd(program string, args ...string) (string, error) {
	return sh.Output(program, args...)
}

func newRunnerFromPackage(packageName string) (*runner, error) {
	r := runner{
		packageName: packageName,
		binaryName:  computeBinaryName(packageName),
		slug:        slugify(packageName),
	}

	if err := r.init(); err != nil {
		return nil, err
	}

	return &r, nil
}

// newRunnerFromSlug creates a runner for an already installed tool, identified
// by the slug of its storage directory under toolDir.
func newRunnerFromSlug(slug string) (*runner, error) {
	r := runner{
		slug: slug,
	}

	if err := r.init(); err != nil {
		return nil, err
	}

	return &r, nil
}

// newRunnerFromBinaryName resolves an installed tool by its binary name (the
// last part of its package path, e.g. "task"). It inspects each installed tool
// and returns the first whose binary name matches. When several tools share a
// binary name the first match wins; they remain stored separately by slug.
func newRunnerFromBinaryName(binaryName string) (*runner, error) {
	slugs, err := installedSlugs()
	if err != nil {
		return nil, err
	}

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

		if computeBinaryName(info.Package) == binaryName {
			r.binaryName = binaryName
			return r, nil
		}
	}

	return nil, fmt.Errorf("magetools: tool %q not found", binaryName)
}

// init resolves the runner's sidecar modfile path. It spawns no subprocess, so
// constructing a runner is free for the read-only paths (Installed, List).
func (r *runner) init() error {
	r.modFile = filepath.Join(toolDir, r.slug, "go.mod")
	return nil
}

// installedSlugs returns the slugs of all installed tools, i.e. the directory
// names directly under toolDir. It returns nil when no tools are installed yet.
func installedSlugs() ([]string, error) {
	entries, err := os.ReadDir(toolDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("magetools: unable to read %s: %w", toolDir, err)
	}

	var slugs []string
	for _, e := range entries {
		if e.IsDir() {
			slugs = append(slugs, e.Name())
		}
	}
	return slugs, nil
}

// slugify turns a package path into a canonical, collision-free directory name
// by dropping any version suffix and replacing every run of characters that
// aren't lowercase letters or digits with a single "-". For example
// "github.com/go-task/task/v3/cmd/task@latest" becomes
// "github-com-go-task-task-v3-cmd-task".
func slugify(packageName string) string {
	p, _, _ := strings.Cut(packageName, "@") // drop @version
	s := slugUnsafe.ReplaceAllString(strings.ToLower(p), "-")
	return strings.Trim(s, "-")
}

// computeBinaryName returns the binary name that `go install <arg>` would produce.
// arg is something like "github.com/go-task/task/v3/cmd/task@latest".
func computeBinaryName(arg string) string {
	p, _, _ := strings.Cut(arg, "@") // drop @version

	name := path.Base(p)
	if majorVersion.MatchString(name) {
		// e.g. "github.com/foo/bar/v2" -> "bar"
		name = path.Base(path.Dir(p))
	}
	return name
}
