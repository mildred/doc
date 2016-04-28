package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	base58 "github.com/jbenet/go-base58"
	attrs "github.com/mildred/doc/attrs"
	ignore "github.com/mildred/doc/ignore"
	repo "github.com/mildred/doc/repo"
)

const usageStatus string = `doc status [OPTIONS...] [DIR]

Scan DIR or the current directory and display a list of new and modified files
with a symbol describing its status:

  ?     Untracked file
  +     Modified file (mtime changed since lash hash)
  *     Unsaved file (missing PAR2 information)
  C     Conflict (main filename)
  c     Conflict (alternate file)

Additionally, (ro) can appear if the file is read only, to notify that doc add
will probably fail to set the extended attributes

Options:
`

func mainStatus(args []string) int {
	f := flag.NewFlagSet("status", flag.ExitOnError)
	opt_no_par2 := f.Bool("n", false, "Do not show files missing PAR2 redundency data")
	opt_show_only_hash := f.Bool("c", false, "Show only unchanged committed files with their hash")
	opt_no_docignore := f.Bool("no-docignore", false, "Don't treat .docignore files specially")
	f.Usage = func() {
		fmt.Print(usageStatus)
		f.PrintDefaults()
	}
	f.Parse(args)
	dir := f.Arg(0)
	if dir == "" {
		dir = "."
	}

	rep := repo.GetRepo(dir)

	status := 0

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
			status = 1
			return err
		}

		// Skip directories containing an empty .docignore file
		if !*opt_no_docignore && ignore.IsIgnored(path) {
			return filepath.SkipDir
		}

		// Skip .dirstore/ at root
		if filepath.Base(path) == attrs.DirStoreName && filepath.Dir(path) == dir && info.IsDir() {
			return filepath.SkipDir
		} else if !info.Mode().IsRegular() {
			return nil
		}

		if *opt_show_only_hash {
			hash, err := repo.GetHash(path, info, false)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
				return nil
			}

			if hash != nil {
				fmt.Printf("%s\t%s\n", base58.Encode(hash), path)
			}

		} else {

			var conflict string = ""
			if repo.ConflictFile(path) != "" {
				conflict = " c"
			} else if len(repo.ConflictFileAlternatives(path)) > 0 {
				conflict = " C"
			}

			hashTime, err := repo.GetHashTime(path)
			if repo.IsNoData(err) {
				if info.Mode()&os.FileMode(0200) == 0 {
					fmt.Printf("?%s (ro)\t%s\n", conflict, path)
				} else {
					fmt.Printf("?%s\t%s\n", conflict, path)
				}
				return nil
			} else if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
				return nil
			}

			var redundency string = "*"
			if rep != nil {
				digest, err := repo.GetHash(path, info, true)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
					return nil
				}
				if par2exists, _ := rep.Par2Exists(digest); par2exists {
					redundency = ""
				}
			}

			if hashTime != info.ModTime() {
				fmt.Printf("+%s%s\t%s\n", conflict, redundency, path)
			} else if conflict != "" || (redundency != "" && !*opt_no_par2) {
				fmt.Printf("%s%s\t%s\n", conflict, redundency, path)
			}
		}

		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}
	return status
}
