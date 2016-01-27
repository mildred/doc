package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	base58 "github.com/jbenet/go-base58"
	attrs "github.com/mildred/doc/attrs"
	repo "github.com/mildred/doc/repo"
)

const checkUsage string = `doc check [OPTIONS...] [DIR]

Scan DIR or the current directory and check for non modified files their content
compared to the stored checksum. If -a is specified, modified files are also
shown.

Symbols show the file status:

  +     Modified file (mtime changed since lash hash)
  =     Modified mtime (identical content but mtime updated)
  !     Corrupt file (mtime identical but content changed)

Options:
`

func mainCheck(args []string) int {
	f := flag.NewFlagSet("status", flag.ExitOnError)
	opt_all := f.Bool("a", false, "Check all files, including modified")
	f.Usage = func() {
		fmt.Print(checkUsage)
		f.PrintDefaults()
	}
	f.Parse(args)
	dir := f.Arg(0)
	if dir == "" {
		dir = "."
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .dirstore/ at root
		if filepath.Base(path) == attrs.DirStoreName && filepath.Dir(path) == dir && info.IsDir() {
			return filepath.SkipDir
		} else if info.IsDir() {
			return nil
		}

		hashTimeStr, err := attrs.Get(path, repo.XattrHashTime)
		if err != nil {
			return nil
		}

		hashTime, err := time.Parse(time.RFC3339Nano, string(hashTimeStr))
		if err != nil {
			return err
		}

		timeEqual := hashTime == info.ModTime()
		if *opt_all || timeEqual {

			hash, err := attrs.Get(path, repo.XattrHash)
			if err != nil {
				return err
			}

			digest, err := repo.HashFile(path, info)
			if err != nil {
				return err
			}

			hashEqual := bytes.Equal(hash, digest)

			if !timeEqual && !hashEqual {
				fmt.Printf("+\t%s\t%s\n", base58.Encode(digest), path)
			} else if !hashEqual {
				fmt.Printf("!\t%s\t%s\n", base58.Encode(digest), path)
			} else if !timeEqual {
				fmt.Printf("=\t%s\t%s", base58.Encode(digest), path)
			}
		}

		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		return 1
	}
	return 0
}
