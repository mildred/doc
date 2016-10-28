package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	base58 "github.com/jbenet/go-base58"
	attrs "github.com/mildred/doc/attrs"
	commit "github.com/mildred/doc/commit"
	ignore "github.com/mildred/doc/ignore"
	repo "github.com/mildred/doc/repo"
)

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
	var c *commit.Commit
	var cDir string
	var err error
	status := 0
	numerr := 0
	doCommit := !opt_nodoccommit

	st, err := os.Lstat(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		return 1
	}

	if st.IsDir() {
		cDir = dir
		c, err = commit.ReadCommit(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s commit: %v\n", dir, err.Error())
			return 1
		}
		c.DropTree("")
	} else {
		cDir = filepath.Dir(dir)
		c, err = commit.ReadCommit(cDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s commit: %v\n", dir, err.Error())
			return 1
		}
		if i, ok := c.ByPath[filepath.Base(dir)]; ok {
			c.Entries[i].DropEntry()
		}
	}

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
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
		} else if !info.IsDir() && !info.Mode().IsRegular() {
			return nil
		}

		relpath, err := filepath.Rel(cDir, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			status = 1
			return err
		}

		if info.IsDir() {
			relpath = relpath + "/"
		}

		hashTime, err := repo.GetHashTime(path)
		if err != nil && !repo.IsNoData(err) {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
			return nil
		}
		hash_is_ok := err == nil && hashTime == info.ModTime()
		var digest []byte

		if hash_is_ok {
			if doCommit {
				digest, err = repo.GetHash(path, info, false)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s\n", err.Error())
					status = 1
					return nil
				} else if digest == nil {
					fmt.Fprintf(os.Stderr, "%s: hash not available\n", path)
					status = 1
					return nil
				}
			}
		} else {
			digest, err = commitFile(path, info, opt_force)
			if err != nil {
				numerr = numerr + 1
				if opt_showerr {
					status = 1
					fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
				}
				return nil
			} else if digest != nil {
				fmt.Printf("%s %s\n", base58.Encode(digest), path)
			}
		}

		if !doCommit {
			return nil
		}

		var uuid string
		var dev, ino uint64
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			uuid = c.UuidByDevInode[commit.DeviceInodeString(st.Dev, st.Ino)]
			dev = st.Dev
			ino = st.Ino
		}

		c.Entries = append(c.Entries, commit.Entry{digest, relpath, uuid, dev, ino, false})
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

	if doCommit {
		err = c.Write()
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
