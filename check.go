package main

import (
  "flag"
  "fmt"
  "os"
  "time"
  "bytes"
  "path/filepath"

  repo "github.com/mildred/doc/repo"
  xattr "github.com/ivaxer/go-xattr"
  base58 "github.com/jbenet/go-base58"
)

func mainCheck(args []string) {
  f := flag.NewFlagSet("status", flag.ExitOnError)
  opt_all := f.Bool("a", false, "Check all files, including modified")
  f.Parse(args)
  dir := f.Arg(0)
  if dir == "" {
    dir = "."
  }

  err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
    if err != nil {
      return err
    }

    // Skip .dirstore/ at root
    if filepath.Base(path) == repo.DirStoreName && filepath.Dir(path) == dir && info.IsDir() {
      return filepath.SkipDir
    } else if info.IsDir() {
      return nil
    }

    hashTimeStr, err := xattr.Get(path, repo.XattrHashTime)
    if err != nil {
      return nil
    }

    hashTime, err := time.Parse(time.RFC3339Nano, string(hashTimeStr))
    if err != nil {
      return err
    }

    timeEqual := hashTime == info.ModTime()
    if *opt_all || timeEqual {

      hash, err := xattr.Get(path, repo.XattrHash)
      if err != nil {
        return err
      }

      digest, err := repo.HashFile(path)
      if err != nil {
        return err
      }

      hashEqual := bytes.Equal(hash, digest)

      if !timeEqual && !hashEqual {
        fmt.Printf("+\t%s\t%s\n", base58.Encode(digest), path)
      } else if !hashEqual {
        fmt.Printf("!\t%s\t%s\n", base58.Encode(digest), path)
      } else if !timeEqual {
        fmt.Printf("=\t%s\t%s", base58.Encode(digest), path)
      }
    }

    return nil
  })

  if err != nil {
    fmt.Fprintf(os.Stderr, "%v", err)
    os.Exit(1)
  }
}

