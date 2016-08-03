package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	attrs "github.com/mildred/doc/attrs"
	commit "github.com/mildred/doc/commit"
)

const initUsage string = `doc init

Creates a .dirstore directory in DIR or the current directory. This will be used
to store PAR2 archives and possibly history information about each file.

In filesystems where extended attributes are not available, it is also used to
store the attributes about each inode.

This directory doesn't store file paths, so you can move this directory freely.
If your filesystem doesn't support extended attributes, don't store this
directory on a different device (as inode numbers are used to associate files to
attributes).

Also, creates an empty .dircommit if there is none.
`

func mainInit(args []string) int {
	f := flag.NewFlagSet("init", flag.ExitOnError)
	f.Usage = func() {
		fmt.Print(initUsage)
		f.PrintDefaults()
	}
	f.Parse(args)
	dir := f.Arg(0)
	if dir == "" {
		dir = "."
	}

	res := 0

	dirstore := path.Join(dir, attrs.DirStoreName)
	err := os.MkdirAll(dirstore, 0777)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		res = res + 1
	}

	err = commit.Init(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		res = res + 1
	}
	return res
}
