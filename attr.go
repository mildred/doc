package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mildred/doc/commit"
	"github.com/mildred/doc/docattr"
)

const usageAttr string = `doc attr [OPTIONS...] [DIR]

Show attributes applied to DIR or the current directory

Options:
`

func mainAttr(args []string) int {
	f := flag.NewFlagSet("status", flag.ExitOnError)
	f.Usage = func() {
		fmt.Print(usageAttr)
		f.PrintDefaults()
	}
	f.Parse(args)
	dir := f.Arg(0)
	if dir == "" {
		dir = "."
	}

	rootDir, prefix, c, err := commit.ReadRootCommit(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	var files []string
	for _, ent := range c.Entries {
		files = append(files, ent.Path)
	}

	attrs, err := docattr.ReadTree(rootDir, prefix, files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	for f, a := range attrs {
		fmt.Println(f)
		for k, v := range a {
			fmt.Printf(" %s=%s\n", k, v)
		}
	}

	return 0
}
