package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const unannexUsage string = `doc unannex [DIR]

Parse DIR and replace all git-annex symbolic links by their content using hard
links. The links are not modified if the object is not available.

Options:
`

func mainUnannex(args []string) int {
	f := flag.NewFlagSet("pull", flag.ExitOnError)
	opt_dry := f.Bool("n", false, "Dry run")
	f.Usage = func() {
		fmt.Print(unannexUsage)
		f.PrintDefaults()
	}
	f.Parse(args)
	dir := "."
	if f.NArg() == 1 {
		dir = f.Arg(0)
	} else if f.NArg() > 1 {
		fmt.Print("Expected one argument maximum")
		return 1
	}

	missing := 0

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
			return err
		}

		// Skip .git
		if filepath.Base(path) == ".git" {
			return filepath.SkipDir
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return unannexSymlink(path, info, *opt_dry, &missing)
		}

		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		return 1
	}

	if missing != 0 {
		fmt.Printf("Missing %d files from the annex\n", missing)
	}

	return 0
}

func unannexSymlink(path string, info os.FileInfo, dry bool, missing *int) error {
	target, err := os.Readlink(path)
	if err != nil {
		return err
	}

	// Return if it is not an annex symlink
	if !strings.Contains(target, ".git/annex/objects/") {
		return nil
	}

	// Return if the target cannot be found
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		fmt.Printf("%s: not in git-annex, leaving it unchanged\n", path)
		*missing += 1
		return nil
	} else if err != nil {
		return err
	}

	targetpath := filepath.Join(filepath.Dir(path), target)

	if dry {
		fmt.Printf("%s: link from %s\n", path, targetpath)
		return nil
	}

	f, err := ioutil.TempFile(filepath.Dir(path), filepath.Base(path))
	if err != nil {
		return err
	}
	fname := f.Name()
	f.Close()
	err = os.Remove(fname)
	if err != nil {
		return err
	}

	err = os.Rename(path, fname)
	if err != nil {
		return err
	}

	err = os.Link(targetpath, path)
	if err != nil {
		return err
	}

	err = os.Remove(fname)
	if err != nil {
		return err
	}

	return nil
}
