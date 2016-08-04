package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"sort"

	commit "github.com/mildred/doc/commit"
)

const usageDiff string = `doc diff [OPTIONS...] [SRC] DEST
doc missing [OPTIONS...] -from SRC [DEST]
doc missing [OPTIONS...] -to DEST [SRC]

Shif differences between STD and DST committed files.

Options:
`

func mainDiff(args []string) int {
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

	var filelist []string
	for file := range srcfiles.ByPath {
		filelist = append(filelist, file)
	}
	for file := range dstfiles.ByPath {
		if _, ok := srcfiles.ByPath[file]; !ok {
			filelist = append(filelist, file)
		}
	}
	sort.Strings(filelist)

	for _, file := range filelist {
		var s, d commit.Entry
		sid, hassrc := srcfiles.ByPath[file]
		did, hasdst := dstfiles.ByPath[file]
		if hassrc {
			s = srcfiles.Entries[sid]
		}
		if hasdst {
			d = dstfiles.Entries[did]
		}
		if hassrc && !hasdst {
			fmt.Printf("- %s\t%s\n", s.HashText(), commit.EncodePath(file))
		} else if !hassrc && hasdst {
			fmt.Printf("+ %s\t%s\n", d.HashText(), commit.EncodePath(file))
		} else if !bytes.Equal(s.Hash, d.Hash) {
			fmt.Printf("- %s\t%s\n", s.HashText(), commit.EncodePath(file))
			fmt.Printf("+ %s\t%s\n", d.HashText(), commit.EncodePath(file))
		}
	}

	return 0
}
