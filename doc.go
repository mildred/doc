package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
)

var commands map[string]func([]string) int

func init() {
	commands = map[string]func([]string) int{
		"help":    mainHelp,
		"init":    mainInit,
		"status":  mainStatus,
		"info":    mainInfo,
		"check":   mainCheck,
		"commit":  mainCommit,
		"cp":      mainCopy,
		"sync":    mainSync,
		"pull":    mainPull,
		"push":    mainPush,
		"save":    mainSave,
		"dupes":   mainDupes,
		"missing": mainMissing,
		"unannex": mainUnannex,
		"diff":    mainDiff,
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

Query commands on files:

        check       Check files integrity with stored checksum
        status      Show status compared to last commit
        info        Show status with detailed informations

Query commands on commit:

        missing     List files missing from a repository compared to another
        diff        Show two way differences between two repositories

Repository commands:

        init        Initialize a repository (defines a root)
        commit      Save current version of files
        save        Save PAR2 redundency information

Synchronisation commands:

        pull        Pull files from another repository that are missing
        push        Push files that are missing in the other repository
        cp          [OLD] scan and copy files one way
        sync        [OLD] scan and copy files two ways

Other commands:

        help        Show help about commands
        dupes       List files that are duplicates
        unannex     Dereference git-annex symlinks
`

var described_commands []string = []string{
	"check", "info", "status", "missing", "diff",
	"init", "commit", "save", "help",
	"cp", "sync", "pull", "push", "unannex", "dupes",
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
