package main

import (
  "flag"
  "fmt"
  "os"
  "path"
  "sort"

  attrs "github.com/mildred/doc/attrs"
)

var commands map[string]func([]string)

func init(){
  commands = map[string]func([]string){
    "help": mainHelp,
    "init": mainInit,
    "status": mainStatus,
    "show": mainShow,
    "check": mainCheck,
    "commit": mainCommit,
    "cp": mainCopy,
    "save": mainSave,
  }
}

func main() {
  //f := flag.NewFlagSet("doc flags")
  flag.Usage = func(){
    mainHelp([]string{})
  }

  flag.Parse()

  f := commands[flag.Arg(0)]
  if f == nil {
    mainHelp(nil)
    os.Exit(1)
  } else {
    f(flag.Args()[1:])
  }
}

const helpText string =
`doc COMMAND ...

doc is a tool to save the status of your files. It record for each file a hash
of its content along with the mtime of the file when the hashing was performed.
It allows you to track file modifications and identity. It can also save PAR2
redundency information about each file (in case they become corrupt).

List of available commands:

`

const helpText2 string = `
You can get help on a command using the -h command line flag or by using the
help command:

        doc COMMAND -h
        doc help COMMAND

`

func mainHelp(args []string) {
  if len(args) == 0 || args[0] == "help" {
    fmt.Printf(helpText)
    var cmds []string
    for cmd, _ := range commands {
      cmds = append(cmds, cmd)
    }
    sort.Strings(cmds)
    for _, cmd := range cmds {
      fmt.Printf("\t%s\n", cmd)
    }
    fmt.Printf(helpText2)
    flag.PrintDefaults()
  } else if cmd, ok := commands[args[0]]; ok {
    cmd([]string{"-h"})
  } else {
    fmt.Fprintf(os.Stderr, "doc %s: command not found\n", args[0])
    os.Exit(1)
  }
}

const initUsage string =
`doc init

Creates a .dirstore directory in DIR or the current directory. This will be used
to store PAR2 archives and possibly history information about each file.

In filesystems where extended attributes are not available, it is also used to
store the attributes about each inode.

This directory doesn't store file paths, so you can move this directory freely.
If your filesystem doesn't support extended attributes, don't store this
directory on a different device (as inode numbers are used to associate files to
attributes).
`

func mainInit(args []string) {
  f := flag.NewFlagSet("init", flag.ExitOnError)
  f.Usage = func(){
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
}

