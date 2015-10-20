package main

import (
  "flag"
  "fmt"
  "os"
  "time"
  "bytes"
  "path/filepath"

  repo "github.com/mildred/doc/repo"
  attrs "github.com/mildred/doc/attrs"
  base58 "github.com/jbenet/go-base58"
)

func mainCommit(args []string) {
  f := flag.NewFlagSet("status", flag.ExitOnError)
  opt_force := f.Bool("f", false, "Force writing xattrs on read only files")
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
    } else if info.IsDir() {
      return nil
    }

    if info.Mode().IsRegular() {
      digest, err := commitFile(path, info, *opt_force)
      if err != nil {
        status = 1
        fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
      } else if digest != nil {
        fmt.Printf("%s %s\n", base58.Encode(digest), path)
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

func commitFile(path string, info os.FileInfo, force bool) ([]byte, error) {
  var forced bool
  digest, err := repo.HashFile(path)
  if err != nil {
    return nil, err
  }

  timeData := []byte(info.ModTime().Format(time.RFC3339Nano))

  hash, err := attrs.Get(path, repo.XattrHash)
  if err != nil || !bytes.Equal(hash, digest) {
    forced, err = attrs.SetForce(path, repo.XattrHash, digest, info, force)
    if forced {
      fmt.Fprintf(os.Stderr, "%s: force write xattrs\n", path)
    }
  } else {
    digest = nil
  }

  hashTimeStr, err := attrs.Get(path, repo.XattrHashTime)
  var hashTime time.Time
  if err == nil {
    hashTime, err = time.Parse(time.RFC3339Nano, string(hashTimeStr))
  }
  if err != nil || hashTime != info.ModTime() {
    forced, err = attrs.SetForce(path, repo.XattrHashTime, timeData, info, force)
    if forced {
      fmt.Fprintf(os.Stderr, "%s: force write xattrs\n", path)
    }
  }

  return digest, err
}