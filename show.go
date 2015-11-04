package main

import (
  "flag"
  "fmt"
  "os"
  "time"
  "bytes"
  "path/filepath"

  mh "github.com/jbenet/go-multihash"
  repo "github.com/mildred/doc/repo"
  attrs "github.com/mildred/doc/attrs"
  base58 "github.com/jbenet/go-base58"
)

const showUsage string =
`doc show [OPTIONS...] FILE...

Show information about each file presented, including its status, hash and
conflict status. It can also run integrity check on the files. It is a more
detailed version of doc status.

Options:
`

func mainShow(args []string) {
  f := flag.NewFlagSet("show", flag.ExitOnError)
  opt_check := f.Bool("c", false, "Run integrity check")
  f.Usage = func(){
    fmt.Print(showUsage)
    f.PrintDefaults()
  }
  f.Parse(args)
  dir := f.Arg(0)
  if dir == "" {
    dir = "."
  }

  rep := repo.GetRepo(dir)

  status := 0
  first := true

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

    if first {
      first = false
    } else {
      fmt.Println()
    }

    fmt.Printf("File: %s\n", path)

    if conflict := repo.ConflictFile(path); conflict != "" {
      fmt.Printf("Conflict With: %s\n", conflict)
    }

    for _, alt := range repo.ConflictFileAlternatives(path) {
      fmt.Printf("Conflict Alternatives: %s\n", alt)
    }

    var realHash mh.Multihash
    if *opt_check {
      realHash, err = repo.HashFile(path)
      if err != nil {
        fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
        return nil
      }
    }

    hashTime, err := repo.GetHashTime(path)

    if repo.IsNoData(err) {
      if *opt_check {
        fmt.Printf("Actual Hash: %s\n", base58.Encode(realHash))
      }
      fmt.Printf("Status: New\n")
    } else {
      if err != nil {
        fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
        return nil
      }

      fmt.Printf("Hash Time: %v\n", hashTime.Format(time.RFC3339Nano))

      hash, err := attrs.Get(path, repo.XattrHash)
      if err != nil {
        fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
        return nil
      }
      var par2exists = false
      if rep != nil {
        par2exists, _ = rep.Par2Exists(hash)
      }
      fmt.Printf("Recorded Hash: %s (reduncency %s)\n", base58.Encode(hash), boolToAvailableStr(par2exists))
      if *opt_check {
        par2exists = false
        if rep != nil {
          par2exists, _ = rep.Par2Exists(realHash)
        }
        fmt.Printf("Actual Hash:   %s (redundency %s)\n", base58.Encode(realHash), boolToAvailableStr(par2exists))
      }

      if hashTime != info.ModTime() {
        fmt.Printf("Status: Dirty\n")
      } else {
        if *opt_check && ! bytes.Equal(realHash, hash) {
          fmt.Printf("Status: Corrupted\n")
        } else {
          fmt.Printf("Status: Clean\n")
        }
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

func boolToAvailableStr(b bool) string {
  if b {
    return "available"
  } else {
    return "unavailable"
  }
}

