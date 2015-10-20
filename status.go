package main

import (
  "flag"
  "fmt"
  "os"
  "path/filepath"

  repo "github.com/mildred/doc/repo"
  attrs "github.com/mildred/doc/attrs"
)

func mainStatus(args []string) {
  f := flag.NewFlagSet("status", flag.ExitOnError)
  f.Parse(args)
  dir := f.Arg(0)
  if dir == "" {
    dir = "."
  }

  status := 0

  err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
    if err != nil {
      fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
      status = 1
      return err
    }

    // Skip .dirstore/ at root
    if filepath.Base(path) == attrs.DirStoreName && filepath.Dir(path) == dir && info.IsDir() {
      return filepath.SkipDir
    } else if ! info.Mode().IsRegular() {
      return nil
    }

    var conflict string = ""
    if repo.ConflictFile(path) != "" {
      conflict = " c"
    } else if len(repo.ConflictFileAlternatives(path)) > 0 {
      conflict = " C"
    }

    hashTime, err := repo.GetHashTime(path)
    if repo.IsNoData(err) {
      if info.Mode() & os.FileMode(0200) == 0 {
        fmt.Printf("?%s (ro)\t%s\n", conflict, path)
      } else {
        fmt.Printf("?%s\t%s\n", conflict, path)
      }
    } else {
      if err != nil {
        fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
        return nil
      }

      if hashTime != info.ModTime() {
        fmt.Printf("+%s\t%s\n", conflict, path)
      } else if conflict != "" {
        fmt.Printf("%s\t%s\n", conflict, path)
      }
    }

    return nil
  })

  if err != nil {
    fmt.Fprintf(os.Stderr, "%v", err)
    os.Exit(1)
  }
  os.Exit(status)
}

