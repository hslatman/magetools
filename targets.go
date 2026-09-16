package magetools

import "fmt"

// Adds a tool
func Add(arg string) error {
	r, err := newRunnerFromPackage(arg) // TODO: add validation?
	if err != nil {
		return err
	}
	return r.get()
}

// Gets all tools
func Get() error {
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
			return err
		}

		// Reinstall the exact version that's pinned in the modfile rather
		// than upgrading to the latest available.
		pkgPath := info.Package
		if info.Version != "" {
			pkgPath += "@" + info.Version
		}
		r.packageName = pkgPath
		r.binaryName = computeBinaryName(info.Package)

		if err := r.get(); err != nil {
			return err
		}
	}

	return nil
}

// Lists all available tools
func List() error {
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

		fmt.Println(computeBinaryName(info.Package))
	}

	return nil
}
