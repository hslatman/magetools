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
	goVersion   string
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

// toolInfo returns the tool's package path and the version it is currently
// pinned at in the modfile. The version is taken from the required module whose
// path is the longest prefix of the tool package path, so that reinstalling a
// tool restores the exact version rather than upgrading it.
func (r *runner) toolInfo() (pkgPath, version string, err error) {
	if !r.modfileExists() {
		return "", "", fmt.Errorf("magetools: no modfile for %s", r.slug)
	}
	content, err := outputCmd("go", "mod", "edit", "-modfile", r.modFile, "-json")
	if err != nil {
		return "", "", fmt.Errorf("magetools: unable to get tool from %s: %w", r.modFile, err)
	}
	var data struct {
		Tools []struct {
			Path string `json:"Path"`
		} `json:"Tool"`
		Require []struct {
			Path    string `json:"Path"`
			Version string `json:"Version"`
		} `json:"Require"`
	}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return "", "", fmt.Errorf(`magetools: unable to parse output of "go mod edit -modfile %s -json": %w`, r.modFile, err)
	}

	if len(data.Tools) == 0 {
		return "", "", fmt.Errorf("magetools: no tool found in %s", r.modFile)
	}

	// NOTE: currently assumes a single tool is present
	pkgPath = data.Tools[0].Path

	best := ""
	for _, req := range data.Require {
		if req.Path == pkgPath || strings.HasPrefix(pkgPath, req.Path+"/") {
			if len(req.Path) > len(best) {
				best = req.Path
				version = req.Version
			}
		}
	}

	return pkgPath, version, nil
}

// TODO: add option to show/debug command that's going to run?
func (r *runner) runCmd(program string, args ...string) error {
	// sh.RunWithV adds os.Environ(); only adds additional env vars here
	additionalEnv := map[string]string{"GOTOOLCHAIN": fmt.Sprintf("go%s", r.goVersion)}
	return sh.RunWithV(additionalEnv, program, args...)
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

		pkgPath, _, err := r.toolInfo()
		if err != nil {
			// Skip directories that aren't valid tool modules.
			continue
		}

		if computeBinaryName(pkgPath) == binaryName {
			r.binaryName = binaryName
			return r, nil
		}
	}

	return nil, fmt.Errorf("magetools: tool %q not found", binaryName)
}

func (r *runner) init() error {
	modFile := filepath.Join(toolDir, r.slug, "go.mod")

	goVersion, err := currentModuleGoVersion()
	if err != nil {
		return err
	}

	r.modFile = modFile
	r.goVersion = goVersion

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

func currentModuleGoVersion() (string, error) {
	content, err := outputCmd("go", "mod", "edit", "-json")
	if err != nil {
		return "", fmt.Errorf("magetools: unable to get current module go version: %w", err)
	}
	var data struct {
		Go string `json:"Go"`
	}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return "", fmt.Errorf(`magetools: unable to parse output of "go mod edit -json": %w`, err)
	}
	return normalizeToolchainVersion(data.Go), nil
}

// normalizeToolchainVersion ensures a Go version is a valid toolchain version.
// The "go" directive in go.mod may be a language version like "1.24", but
// GOTOOLCHAIN requires a full toolchain version like "go1.24.0" ("go1.24" is
// rejected as "a language version but not a toolchain version"). A "major.minor"
// version therefore gets a ".0" patch appended; anything else is left as-is.
func normalizeToolchainVersion(version string) string {
	if strings.Count(version, ".") == 1 {
		return version + ".0"
	}
	return version
}
