package magetools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUpdateUpgradesPinnedVersion installs an old version of a real tool into a
// temporary tool directory and asserts that Update moves it forward. It shells
// out to the go command and may hit the module proxy, so it is skipped under
// -short.
func TestUpdateUpgradesPinnedVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: installs a tool via the go command")
	}

	// The go command resolves -modfile relative to the working directory, and
	// refuses to use it at all outside a main module ("cannot find main module,
	// but -modfile was set"), so the test needs a real module to run in.
	t.Chdir(t.TempDir())

	if _, err := outputCmd("go", "mod", "init", "example.com/updatetest"); err != nil {
		t.Fatalf("go mod init: %v", err)
	}

	setToolDir(t, ".magetools")

	const (
		pkg = "github.com/magefile/mage"
		old = "v1.15.0"
	)

	if err := Add(pkg + "@" + old); err != nil {
		t.Fatalf("Add(%s@%s): %v", pkg, old, err)
	}

	before := installedVersion(t, pkg)
	if before != old {
		t.Fatalf("after Add, version = %q, want %q", before, old)
	}

	if err := Update(); err != nil {
		t.Fatalf("Update(): %v", err)
	}

	after := installedVersion(t, pkg)
	if after == old {
		t.Fatalf("after Update, version = %q, want something newer than %q", after, old)
	}
	if after == "" {
		t.Fatal("after Update, version is empty")
	}
	t.Logf("upgraded %s %s => %s", pkg, before, after)
}

// TestUpdateNoTools is the empty case: nothing installed is not an error.
func TestUpdateNoTools(t *testing.T) {
	setToolDir(t, filepath.Join("testdata", "does-not-exist"))

	if err := Update(); err != nil {
		t.Fatalf("Update() with no tools: %v, want nil", err)
	}
}

// installedVersion reads back the version Installed reports for a package whose
// path is pkg.
func installedVersion(t *testing.T, pkg string) string {
	t.Helper()
	tools, err := Installed()
	if err != nil {
		t.Fatalf("Installed(): %v", err)
	}
	for _, tool := range tools {
		if tool.Package == pkg {
			return tool.Version
		}
	}
	t.Fatalf("Installed() does not contain %s: %+v", pkg, tools)
	return ""
}

// TestUpdateUsesModulePathNotPackagePath pins down which of the two paths in a
// tool modfile Update hands to "go get -u": the module path, not the tool
// package path. TestUpdateUpgradesPinnedVersion cannot tell them apart, because
// its tool (github.com/magefile/mage) has a module path identical to its
// package path — so substituting info.Package for info.Module at the call site
// would keep every other test green while breaking any /v2-style tool in the
// field.
//
// The check is offline. With GOPROXY=off the go command refuses the lookup and
// reports the module path it was asked for, and mage's sh.RunWithV embeds the
// full argument list in its error, so Update's returned error spells out
// exactly what it passed.
func TestUpdateUsesModulePathNotPackagePath(t *testing.T) {
	const (
		modulePath  = "zz.example.com/mod"
		packagePath = "zz.example.com/mod/cmd/tool"
		slug        = "zz-example-com-mod-cmd-tool"
	)

	// The go command refuses -modfile outside a main module, so Update needs a
	// real module as its working directory.
	t.Chdir(t.TempDir())

	if _, err := outputCmd("go", "mod", "init", "example.com/updatetest"); err != nil {
		t.Fatalf("go mod init: %v", err)
	}

	setToolDir(t, ".magetools")
	t.Setenv("GOPROXY", "off")

	dir := filepath.Join(".magetools", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating %s: %v", dir, err)
	}
	modfile := "module tool\n\ngo 1.25.0\n\ntool " + packagePath + "\n\nrequire " + modulePath + " v1.0.0 // indirect\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(modfile), 0o644); err != nil {
		t.Fatalf("writing tool modfile: %v", err)
	}

	err := Update()
	if err == nil {
		t.Fatal("Update() = nil error, want a failed lookup under GOPROXY=off")
	}

	got := err.Error()
	if !strings.Contains(got, modulePath) {
		t.Errorf("Update() error does not mention the module path %q: %v", modulePath, got)
	}
	// modulePath is a prefix of packagePath, so the check above cannot rule the
	// package path out; it needs its own assertion.
	if strings.Contains(got, packagePath) {
		t.Errorf("Update() error mentions the package path %q, want the module path: %v", packagePath, got)
	}
}
