package magetools

import "fmt"

// Updates all tools to their latest available version
//
// Update is the only operation in this package that moves a version. Add
// installs, Get reinstalls whatever is pinned, and List lists; a tool's version
// changes only when Update is run, so a repository's pins never drift on their
// own. Each tool's sidecar modfile is rewritten in place and nothing else on
// disk is touched.
//
// Each tool is upgraded with "go get -u" applied to the module providing it,
// which has two consequences worth knowing. It upgrades that module and its
// dependencies to their latest minor or patch releases, so more require lines
// in the sidecar modfile can move than the tool's own pin. And it never crosses
// a major version: a tool pinned at /v2 is never moved to /v3, because that
// would also change the package path in the "tool" directive. Crossing a major
// version means adding the new path with Add.
func Update() error {
	slugs, err := installedSlugs()
	if err != nil {
		return err
	}

	for _, slug := range slugs {
		r, err := newRunnerFromSlug(slug)
		if err != nil {
			return err
		}

		info, err := r.toolInfo()
		if err != nil {
			// Skip directories that aren't valid tool modules.
			continue
		}

		if info.Module == "" {
			// A tool directive with no matching require: nothing to upgrade.
			continue
		}

		// "go get -u" is applied to the module that provides the tool, which is
		// the require entry toolInfo resolved by longest-prefix match.
		if err := r.runCmd("go", "get", "-modfile", r.modFile, "-u", info.Module); err != nil {
			return fmt.Errorf("magetools: unable to update %s: %w", slug, err)
		}
	}

	return nil
}
