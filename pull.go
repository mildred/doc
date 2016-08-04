package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mildred/doc/copy"
	"golang.org/x/crypto/ssh/terminal"
)

const pullPushUsage string = `doc pull [OPTIONS...] SRC [TARGET]
doc push [OPTIONS...] [SRC] TARGET

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
	opt_quiet := f.Bool("q", false, "Quiet about attribute errors")
	opt_verbose := f.Bool("v", false, "Print a log of operations")
	f.Usage = func() {
		fmt.Print(pullPushUsage)
		f.PrintDefaults()
	}
	f.Parse(args)
	var src, target string
	if f.NArg() == 1 {
		target = "."
		src = f.Arg(0)
	} else if f.NArg() == 2 {
		src = f.Arg(0)
		target = f.Arg(1)
	} else {
		fmt.Print("Expected one or two arguments")
		return 1
	}

	return pullPush(src, target, *opt_quiet, *opt_verbose)
}

func mainPush(args []string) int {
	f := flag.NewFlagSet("pull", flag.ExitOnError)
	opt_quiet := f.Bool("q", false, "Quiet about attribute errors")
	opt_verbose := f.Bool("v", false, "Print a log of operations")
	f.Usage = func() {
		fmt.Print(pullPushUsage)
		f.PrintDefaults()
	}
	f.Parse(args)
	var src, target string
	if f.NArg() == 1 {
		src = "."
		target = f.Arg(0)
	} else if f.NArg() == 2 {
		src = f.Arg(0)
		target = f.Arg(1)
	} else {
		fmt.Print("Expected one or two arguments")
		return 1
	}

	return pullPush(src, target, *opt_quiet, *opt_verbose)
}

func pullPush(src, target string, quiet bool, verb bool) int {
	p := newPullProgress(verb)

	res := 0
	err, errs := copy.Copy(src, target, p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		res = 1
	}
	if !quiet && len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "\n")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "W: %s\n", e.Error())
		}
	}
	return res
}

type pullProgress struct {
	first   bool
	lastmsg string
	verbose bool
}

func newPullProgress(verb bool) *pullProgress {
	return &pullProgress{true, "", verb}
}

func (p *pullProgress) SetProgress(cur, max int, msg string) {
	cols, _, err := terminal.GetSize(0)
	if err != nil {
		format := fmt.Sprintf("%%%dd", len(fmt.Sprintf("%d", max)))
		fmt.Printf(format+"/%d %s\n", cur, max, msg)
	} else {
		if p.first {
			p.first = false
		} else if p.verbose {
			fmt.Printf("\x1b[2A\x1b[K%s\n", p.lastmsg)
		} else {
			fmt.Print("\x1b[2A")
		}
		percent := fmt.Sprintf("%d%%", 100*cur/max)
		ratio := fmt.Sprintf("%d/%d", cur, max)
		progresssize := cols - len(percent) - 2
		psz1 := progresssize * cur / max
		psz2 := progresssize - psz1
		progress := strings.Repeat("#", psz1) + strings.Repeat("-", psz2)
		msglen := cols - len(ratio) - 2
		if len(msg) > msglen {
			msg = msg[:msglen]
		}
		p.lastmsg = fmt.Sprintf("%s %s", ratio, msg)
		fmt.Printf("\x1b[K%s %s\n\x1b[K%s\n", percent, progress, p.lastmsg)
	}
}
