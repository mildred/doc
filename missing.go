package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	commit "github.com/mildred/doc/commit"
)

const usageMissing string = `doc missing [OPTIONS...] [SRC] DEST
doc missing [OPTIONS...] -from SRC [DEST]
doc missing [OPTIONS...] -to DEST [SRC]

Show files in SRC that are different from the same files in DEST (only committed
changes). Lines start with a symbol, followed by the file hash and the file
name.

  -     Represents the file in SRC
  +     Represents the file in DEST

Conflicts are represented by two lines (one for each version).

Options:
`

func mainMissing(args []string) int {
	f := flag.NewFlagSet("status", flag.ExitOnError)
	opt_from := f.String("from", "", "Specify the source directory")
	opt_to := f.String("to", "", "Specify the destination directory")
	f.Usage = func() {
		fmt.Print(usageMissing)
		f.PrintDefaults()
	}
	f.Parse(args)
	src, dst := findSourceDest(*opt_from, *opt_to, f.Args())

	srcfiles, err := commit.ReadCommit(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s", src, err.Error())
	}

	dstfiles, err := commit.ReadCommit(dst)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s", dst, err.Error())
	}

	for _, s := range srcfiles.Entries {
		did, hasd := dstfiles.ByPath[s.Path]
		if !hasd {
			fmt.Printf("- %s\t%s\n", s.HashText(), commit.EncodePath(s.Path))
		} else {
			d := dstfiles.Entries[did]
			if !bytes.Equal(s.Hash, d.Hash) {
				fmt.Printf("- %s\t%s\n", s.HashText(), commit.EncodePath(s.Path))
				fmt.Printf("+ %s\t%s\n", d.HashText(), commit.EncodePath(d.Path))
			}
		}
	}

	return 0
}
