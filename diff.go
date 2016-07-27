package main

import (
	"flag"
	"fmt"
	commit "github.com/mildred/doc/commit"
	"os"
	"path/filepath"
	"sort"
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

	srccommit := filepath.Join(src, ".doccommit")
	dstcommit := filepath.Join(dst, ".doccommit")

	srcfiles, err := commit.ReadByPath(srccommit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s", srccommit, err.Error())
	}

	dstfiles, err := commit.ReadByPath(dstcommit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s", dstcommit, err.Error())
	}

	var filelist []string
	for file := range srcfiles {
		filelist = append(filelist, file)
	}
	for file := range dstfiles {
		if _, ok := srcfiles[file]; !ok {
			filelist = append(filelist, file)
		}
	}
	sort.Strings(filelist)

	for _, file := range filelist {
		shash, hassrc := srcfiles[file]
		dhash, hasdst := dstfiles[file]
		if hassrc && hasdst && shash != dhash {
			fmt.Printf("- %s\t%s\n", shash, commit.EncodePath(file))
			fmt.Printf("+ %s\t%s\n", shash, commit.EncodePath(file))
		} else if hassrc && !hasdst {
			fmt.Printf("- %s\t%s\n", shash, commit.EncodePath(file))
		} else if !hassrc && hasdst {
			fmt.Printf("+ %s\t%s\n", dhash, commit.EncodePath(file))
		}
	}

	return 0
}
