package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"sort"

	attrs "github.com/mildred/doc/attrs"
)

var commands map[string]func([]string) int

func init() {
	commands = map[string]func([]string) int{
		"help":    mainHelp,
		"init":    mainInit,
		"status":  mainStatus,
		"show":    mainShow,
		"check":   mainCheck,
		"commit":  mainCommit,
		"cp":      mainCopy,
		"sync":    mainSync,
		"save":    mainSave,
		"dupes":   mainDupes,
		"missing": mainMissing,
	}
}

func main() {
	//f := flag.NewFlagSet("doc flags")
	flag.Usage = func() {
		mainHelp([]string{})
	}

	flag.Parse()

	f := commands[flag.Arg(0)]
	if f == nil {
		mainHelp(nil)
		os.Exit(1)
	} else {
		status := f(flag.Args()[1:])
		if status != 0 {
			os.Exit(status)
		}
	}
}

const helpText string = `doc COMMAND ...

doc is a tool to save the status of your files. It record for each file a hash
of its content along with the mtime of the file when the hashing was performed.
It allows you to track file modifications and identity. It can also save PAR2
redundency information about each file (in case they become corrupt).

Query commands:

        check
        show
        status
        missing

Repository commands:

        init
        commit
        save

Synchronisation commands:

        cp
        sync

Other commands:

`

var described_commands []string = []string{
	"check", "show", "status", "missing",
	"init", "commit", "save",
	"cp", "sync",
}

const helpText2 string = `
You can get help on a command using the -h command line flag or by using the
help command:

        doc COMMAND -h
        doc help COMMAND

`

func mainHelp(args []string) int {
	if len(args) == 0 || args[0] == "help" {
		fmt.Printf(helpText)
		var cmds []string
		for cmd, _ := range commands {
			cmds = append(cmds, cmd)
		}
		sort.Strings(cmds)
		for _, cmd := range cmds {
			already_described := false
			for _, e := range described_commands {
				if e == cmd {
					already_described = true
					break
				}
			}
			if !already_described {
				fmt.Printf("\t%s\n", cmd)
			}
		}
		fmt.Printf(helpText2)
		flag.PrintDefaults()
	} else if cmd, ok := commands[args[0]]; ok {
		cmd([]string{"-h"})
	} else {
		fmt.Fprintf(os.Stderr, "doc %s: command not found\n", args[0])
		return 1
	}
	return 0
}

const initUsage string = `doc init

Creates a .dirstore directory in DIR or the current directory. This will be used
to store PAR2 archives and possibly history information about each file.

In filesystems where extended attributes are not available, it is also used to
store the attributes about each inode.

This directory doesn't store file paths, so you can move this directory freely.
If your filesystem doesn't support extended attributes, don't store this
directory on a different device (as inode numbers are used to associate files to
attributes).
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

	dirstore := path.Join(dir, attrs.DirStoreName)
	os.Mkdir(dirstore, 0777)
	return 1
}

func findSourceDest(opt_src, opt_dst string, args []string) (src string, dst string) {
	var arg0, arg1 string
	if len(args) > 0 {
		arg0 = args[0]
	}
	if len(args) > 1 {
		arg1 = args[1]
	}
	src = opt_src
	dst = opt_dst
	if src == "" && dst == "" {
		src = arg0
		dst = arg1
		if dst == "" {
			dst = src
			src = "."
		}
	} else if dst == "" {
		dst = arg0
		if dst == "" {
			dst = "."
		}
	} else if src == "" {
		src = arg0
		if src == "" {
			src = "."
		}
	}

	if src == "" || dst == "" {
		fmt.Fprintln(os.Stderr, "You must specify at least the destination directory")
		os.Exit(1)
	}

	return
}
