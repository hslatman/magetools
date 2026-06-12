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

type runner struct {
	packageName string
	binaryName  string
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

type tool struct {
	Path string `json:"Path"`
}

func (r *runner) tool() (string, error) {
	if !r.modfileExists() {
		return "", fmt.Errorf("magetools: no modfile for %s", r.binaryName)
	}
	content, err := outputCmd("go", "mod", "edit", "-modfile", r.modFile, "-json")
	if err != nil {
		return "", fmt.Errorf("magetools: unable to get tool from %s: %w", r.modFile, err)
	}
	var data struct {
		Tools []tool `json:"Tool"`
	}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return "", fmt.Errorf(`magetools: unable to parse output of "go mod edit -modfile %s -json": %w`, r.modFile, err)
	}

	if len(data.Tools) == 0 {
		return "", fmt.Errorf("magetools: no tool found in %s", r.modFile)
	}

	// NOTE: currently assumes a single tool is present
	return data.Tools[0].Path, nil
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
	}

	if err := r.init(); err != nil {
		return nil, err
	}

	return &r, nil
}

func newRunnerFromBinaryName(binaryName string) (*runner, error) {
	r := runner{
		binaryName: binaryName,
	}

	if err := r.init(); err != nil {
		return nil, err
	}

	return &r, nil
}

func (r *runner) init() error {
	modFile := filepath.Join(toolDir, r.binaryName, "go.mod")

	goVersion, err := currentModuleGoVersion()
	if err != nil {
		return err
	}

	r.modFile = modFile
	r.goVersion = goVersion

	return nil
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
	return data.Go, nil
}
