package magetools

import (
	"fmt"
	"io/fs"
	"os"
)

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
	root, err := os.OpenRoot(toolDir)
	if err != nil {
		return err
	}

	fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if path == "." {
			return nil
		}

		if !d.IsDir() {
			return nil
		}

		r, err := newRunnerFromBinaryName(path)
		if err != nil {
			return err
		}

		tool, err := r.tool()
		if err != nil {
			return err
		}

		r.packageName = tool

		if err := r.get(); err != nil {
			return err
		}

		return nil
	})

	return nil
}

// Lists all available tools
func List() error {
	root, err := os.OpenRoot(toolDir)
	if err != nil {
		return err
	}

	fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if path == "." {
			return nil
		}

		if !d.IsDir() {
			return nil
		}

		// TODO: additional validation; information?
		fmt.Println(path)

		return nil
	})

	return nil
}
