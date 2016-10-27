package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	base58 "github.com/jbenet/go-base58"
	attrs "github.com/mildred/doc/attrs"
	commit "github.com/mildred/doc/commit"
	ignore "github.com/mildred/doc/ignore"
	repo "github.com/mildred/doc/repo"
)

const FIXME_Uuid = ""

const commitUsage string = `doc commit [OPTIONS...] [DIR]

For each modified file in DIR or the current directory, computes a checksum and
store it in the extended attributes.

Writes in a file named .doccommit in each directory the commit summary (list of
file, and for each file its hash and timestamp). If the file is manually
modified, this will be detected and it will not be overwritten.

Options:
`

func mainCommit(args []string) int {
	f := flag.NewFlagSet("commit", flag.ExitOnError)
	opt_force := f.Bool("f", false, "Force writing xattrs on read only files")
	opt_nodoccommit := f.Bool("n", false, "Don't write .doccommit")
	opt_nodocignore := f.Bool("no-docignore", false, "Don't respect .docignore")
	opt_showerr := f.Bool("e", false, "Show individual errors")
	f.Usage = func() {
		fmt.Print(commitUsage)
		f.PrintDefaults()
	}
	f.Parse(args)

	if len(f.Args()) == 0 {
		return runCommit(".", *opt_force, *opt_nodoccommit, *opt_nodocignore, *opt_showerr)
	} else {
		status := 0
		for _, arg := range f.Args() {
			status = status + runCommit(arg, *opt_force, *opt_nodoccommit, *opt_nodocignore, *opt_showerr)
		}
		return status
	}
}

func runCommit(dir string, opt_force, opt_nodoccommit, opt_nodocignore, opt_showerr bool) int {
	status := 0
	numerr := 0

	var commitEntries []commit.Entry
	doCommit := !opt_nodoccommit

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
			status = 1
			return err
		}

		relpath, err := filepath.Rel(dir, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			status = 1
			return err
		}

		if !opt_nodocignore && ignore.IsIgnored(path) {
			return filepath.SkipDir
		}

		// Skip .dirstore/ at root and .doccommit
		if filepath.Base(path) == attrs.DirStoreName && filepath.Dir(path) == dir && info.IsDir() {
			return filepath.SkipDir
		} else if doCommit && filepath.Join(dir, commit.Doccommit) == path {
			return nil
		} else if info.IsDir() {
			relpath = relpath + "/"
		} else if !info.Mode().IsRegular() {
			return nil
		}

		hashTime, err := repo.GetHashTime(path)
		if err != nil && !repo.IsNoData(err) {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
			return nil
		}

		if err == nil && hashTime == info.ModTime() {
			if doCommit {
				hash, err := repo.GetHash(path, info, false)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s\n", err.Error())
					status = 1
				} else if hash == nil {
					fmt.Fprintf(os.Stderr, "%s: hash not available\n", path)
					status = 1
				} else {
					commitEntries = append(commitEntries, commit.Entry{hash, relpath, FIXME_Uuid})
				}
			}
		} else {
			digest, err := commitFile(path, info, opt_force)
			if err != nil {
				numerr = numerr + 1
				if opt_showerr {
					status = 1
					fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
				}
			} else if digest != nil {
				fmt.Printf("%s %s\n", base58.Encode(digest), path)
			}
			if doCommit {
				commitEntries = append(commitEntries, commit.Entry{digest, relpath, FIXME_Uuid})
			}
		}

		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		status = 1
	}

	if numerr > 0 {
		if dir == "." || dir == "" {
			fmt.Fprintf(os.Stderr, "%d errors when writing extended attributes (probably read only files)\n", numerr)
		} else {
			fmt.Fprintf(os.Stderr, "%s: %d errors when writing extended attributes (probably read only files)\n", dir, numerr)
		}
	}

	if doCommit && len(commitEntries) > 0 {
		st, err := os.Lstat(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			return 1
		}
		if !st.IsDir() && len(commitEntries) == 1 {
			commitEntries[0].Path = filepath.Base(dir)
			dir = filepath.Dir(dir)
			err = commit.WriteDirAppend(dir, commitEntries)
		} else {
			err = commit.WriteDir(dir, commitEntries)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			status = 1
		}
	}

	return status
}

func commitFile(path string, info os.FileInfo, force bool) ([]byte, error) {
	var forced bool
	digest, err := repo.HashFile(path, info)
	if err != nil {
		return nil, err
	}

	forced, err = repo.CommitFileHash(path, info, digest, force)
	if forced {
		fmt.Fprintf(os.Stderr, "%s: force write xattrs\n", path)
	}

	return digest, err
}
