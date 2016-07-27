package main

import (
	"flag"
	"fmt"

	"github.com/mildred/doc/sync"
)

const pullPushUsage string = `doc pull [OPTIONS...] [SRC] DEST
doc push [OPTIONS...] [SRC] DEST

Read the .doccommit file in the current directory and the distant directory,
then synchronize changes with DEST. Files that are different are marked in
conflict, and new files are added.

Before copying, no check is performed to make sure that the file has not been
modified since last commit. It is assumed that no file is modified.

You should run doc commit on the destination directory afterwards.

Options:
`

func mainPull(args []string) int {
	f := flag.NewFlagSet("pull", flag.ExitOnError)
	opt_dry := f.Bool("n", false, "Dry run")
	opt_force := f.Bool("f", false, "Force copy even if there are errors")
	opt_quiet := f.Bool("q", false, "Quiet")
	f.Usage = func() {
		fmt.Print(pullPushUsage)
		f.PrintDefaults()
	}
	f.Parse(args)
	var loc, dist string
	if f.NArg() == 1 {
		loc = "."
		dist = f.Arg(0)
	} else if f.NArg() == 2 {
		loc = f.Arg(0)
		dist = f.Arg(1)
	} else {
		fmt.Print("Expected two arguments")
		return 1
	}

	sync_opts := sync.SyncOptions{
		Preparator: &sync.CommitPreparatorOpts{},
		DryRun:     *opt_dry,
		Force:      *opt_force,
		Quiet:      *opt_quiet,
		Dedup:      false,
		DeleteDup:  false,
		TwoPass:    true,
	}
	if sync.Sync(dist, loc, sync_opts) > 0 {
		return 1
	}

	return 0
}

func mainPush(args []string) int {
	f := flag.NewFlagSet("pull", flag.ExitOnError)
	opt_dry := f.Bool("n", false, "Dry run")
	opt_force := f.Bool("f", false, "Force copy even if there are errors")
	opt_quiet := f.Bool("q", false, "Quiet")
	f.Usage = func() {
		fmt.Print(pullPushUsage)
		f.PrintDefaults()
	}
	f.Parse(args)
	var loc, dist string
	if f.NArg() == 1 {
		loc = "."
		dist = f.Arg(0)
	} else if f.NArg() == 2 {
		loc = f.Arg(0)
		dist = f.Arg(1)
	} else {
		fmt.Print("Expected two arguments")
		return 1
	}

	sync_opts := sync.SyncOptions{
		Preparator: &sync.CommitPreparatorOpts{},
		DryRun:     *opt_dry,
		Force:      *opt_force,
		Quiet:      *opt_quiet,
		Dedup:      false,
		DeleteDup:  false,
		TwoPass:    true,
	}
	if sync.Sync(loc, dist, sync_opts) > 0 {
		return 1
	}

	return 0
}
