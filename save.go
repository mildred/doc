package main

import (
  "os"
  "fmt"
  "flag"
  "path/filepath"

  repo "github.com/mildred/doc/repo"
  attrs "github.com/mildred/doc/attrs"
  base58 "github.com/jbenet/go-base58"
)

const saveUsage string =
`doc save [FILE]

For each modified file in DIR or the current directory, computes a checksum and
store it in the extended attributes. A PAR2 archive is also created and stored
separately in the .dirstore directory.

Options:
`

func mainSave(args []string) int {
  f := flag.NewFlagSet("save", flag.ExitOnError)
  opt_force := f.Bool("force", false, "Force writing xattrs on read only files")
  f.Usage = func(){
    fmt.Print(saveUsage)
    f.PrintDefaults()
  }
  f.Parse(args)
  dir := f.Arg(0)
  if dir == "" {
    dir = "."
  }

  dirstore := repo.GetRepo(dir)
  if dirstore == nil {
    fmt.Fprintf(os.Stderr, "%s: Could not find repository, please run doc init\n", dir)
    os.Exit(1)
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
    } else if info.IsDir() || ! info.Mode().IsRegular() {
      return nil
    }

    hashTime, err := repo.GetHashTime(path)
    if err != nil && ! repo.IsNoData(err) {
      fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
      return nil
    }

    var digest []byte

    if err != nil || hashTime != info.ModTime() {
      digest, err = commitFile(path, info, *opt_force)
      if err != nil {
        status = 1
        fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
      } else if digest != nil {
        fmt.Printf("%s %s\n", base58.Encode(digest), path)
      }
    } else {
      digest, err = attrs.Get(path, repo.XattrHash)
      if err != nil {
        fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
        return nil
      }
    }

	err = dirstore.Create(path, digest)
    if err != nil {
      fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
      return nil
    }

    return nil
  })

  if err != nil {
    fmt.Fprintf(os.Stderr, "%v", err)
    os.Exit(1)
  }

  return status
}

