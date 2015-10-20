package main

import (
  "flag"
  "fmt"
  "os"
  "path"

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
  }
}

func main() {
  //f := flag.NewFlagSet("doc flags")
  flag.Parse()

  f := commands[flag.Arg(0)]
  if f == nil {
    mainHelp(nil)
  } else {
    f(flag.Args()[1:])
  }
}

func mainHelp([]string) {
  for cmd, _ := range commands {
    fmt.Printf("%s\n", cmd)
  }
}

func mainInit(args []string) {
  f := flag.NewFlagSet("init", flag.ExitOnError)
  f.Parse(args)
  dir := f.Arg(0)
  if dir == "" {
    dir = "."
  }

  dirstore := path.Join(dir, attrs.DirStoreName)
  os.Mkdir(dirstore, 0777)
}

