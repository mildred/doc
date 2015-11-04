package main

import (
  "os"
  "fmt"
  "flag"
  "syscall"

  repo "github.com/mildred/doc/repo"
  base58 "github.com/jbenet/go-base58"
)

const dupesUsage string =
`doc dupes [DIR]

List duplicate files under the given or the current directory. Group of
identical files are separated by a blank line. The number of detected links is
shown in front of each file.

Options:
`

type sameFile struct {
  hash   []byte
  paths  []string
  inodes []uint64
}

func mainDupes(args []string) {
  f := flag.NewFlagSet("dupes", flag.ExitOnError)
  opt_show_links := f.Bool("l", false, "Show group of files that share the same inode")
  opt_progress := f.Bool("p", false, "Show progress")
  f.Usage = func(){
    fmt.Print(dupesUsage)
    f.PrintDefaults()
  }
  f.Parse(args)
  src := f.Arg(0)

  if src == "" {
    src = "."
  }

  dupes := map[string]sameFile{}

  num := 0

  e := repo.Walk(src, func(path string, info os.FileInfo)error{

    hash, err := repo.GetHash(path, info)
    if err != nil {
      return err
    }

    sys, ok := info.Sys().(*syscall.Stat_t)
    if ! ok {
      sys.Ino = 0
    }

    f := dupes[string(hash)]
    f.hash = hash
    f.paths = append(f.paths, path)
    f.inodes = append(f.inodes, sys.Ino)
    dupes[string(hash)] = f

    num = num + 1
    if *opt_progress {
      fmt.Printf("\r\x1b[2K%d %s\r", num, path)
    }

    return nil
  }, func(path string, info os.FileInfo, err error)bool{
    fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
    return true
  })

  for _, f := range dupes {
    if len(f.paths) <= 1 {
      continue
    }
    files := map[uint64][]string{}
    for i, ino := range f.inodes {
      files[ino] = append(files[ino], f.paths[i])
    }
    if len(files) == 1 && !*opt_show_links {
      continue
    }
    fmt.Println()
    hash := base58.Encode(f.hash)
    for _, paths := range files {
      for _, path := range paths {
        fmt.Printf("%s\t%d\t%s\n", hash, len(paths), path)
      }
    }
  }

  if e != nil {
    os.Exit(1)
  }
}

