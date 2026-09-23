package magetools

import (
	"path/filepath"
	"testing"
)

// runCmd must not impose a GOTOOLCHAIN on the commands it runs.
//
// Pinning it to the application's go directive makes a tool whose own go.mod
// requires a newer Go impossible to install, and pinning it to a "+auto" form
// instead makes the go command re-exec into a different toolchain, which
// `go get -modfile` does not survive. Either way the pin is what breaks
// installing current tools, so the child must see whatever the caller set.
func TestRunCmdDoesNotOverrideGOTOOLCHAIN(t *testing.T) {
	t.Setenv("GOTOOLCHAIN", "local")

	r := &runner{binaryName: "probe", modFile: filepath.Join(t.TempDir(), "go.mod")}

	// sh -c exits 0 only if the child saw the caller's value untouched. "local" is a valid
	// setting, so any lookup the pin needs still succeeds and the test isolates
	// the override rather than tripping on an invalid value.
	if err := r.runCmd("sh", "-c", `test "$GOTOOLCHAIN" = "local"`); err != nil {
		t.Errorf("child did not see the caller's GOTOOLCHAIN: %v", err)
	}
}
