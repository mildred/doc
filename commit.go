package main

import (
  "flag"
  "fmt"
  "os"
  "path/filepath"

  repo "github.com/mildred/doc/repo"
  attrs "github.com/mildred/doc/attrs"
  commit "github.com/mildred/doc/commit"
  base58 "github.com/jbenet/go-base58"
)

const commitUsage string =
`doc commit [OPTIONS...] [DIR]

For each modified file in DIR or the current directory, computes a checksum and
store it in the extended attributes.

Writes in a file named .doccommit in each directory the commit summary (list of
file, and for each file its hash and timestamp). If the file is manually
modified, this will be detected and it will not be overwritten.

Options:
`

func mainCommit(args []string) {
  f := flag.NewFlagSet("commit", flag.ExitOnError)
  opt_force := f.Bool("f", false, "Force writing xattrs on read only files")
  opt_nodoccommit := f.Bool("n", false, "Don't write .doccommit")
  f.Usage = func(){
    fmt.Print(commitUsage)
    f.PrintDefaults()
  }
  f.Parse(args)
  dir := f.Arg(0)
  if dir == "" {
    dir = "."
  }

  status := 0

  var doccommit *commit.CommitFileWriter
  if ! *opt_nodoccommit {
    var err error
    doccommit, err = commit.Create(filepath.Join(dir, ".doccommit"))
    if err != nil {
      fmt.Fprintf(os.Stderr, "%v\n", err)
      os.Exit(1)
    }
  }

  err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
    if err != nil {
      fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
      status = 1
      return err
    }

    relpath, err := filepath.Rel(dir, path)
    if err != nil {
      fmt.Fprintf(os.Stderr, "%s\n", err.Error())
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

    if err == nil && hashTime == info.ModTime() {
      if doccommit != nil {
        hash, err := repo.GetHash(path, info, false)
        if err != nil {
          fmt.Fprintf(os.Stderr, "%s\n", err.Error())
          status = 1
        } else if hash == nil {
          fmt.Fprintf(os.Stderr, "%s: hash not available\n", path)
          status = 1
        } else {
          err := doccommit.AddEntry(hash, relpath)
          if err != nil {
            fmt.Fprintf(os.Stderr, "%s", err.Error())
            status = 1
          }
        }
      }
    } else {
      digest, err := commitFile(path, info, *opt_force)
      if err != nil {
        status = 1
        fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
      } else if digest != nil {
        fmt.Printf("%s %s\n", base58.Encode(digest), path)
      }
      if doccommit != nil {
        err := doccommit.AddEntry(digest, relpath)
        if err != nil {
          fmt.Fprintf(os.Stderr, "%s\n", err.Error())
          status = 1
        }
      }
    }

    return nil
  })

  if err != nil {
    fmt.Fprintf(os.Stderr, "%v\n", err)
    status = 1
  }

  if doccommit != nil {
    err := doccommit.Close()
    if err != nil {
      fmt.Fprintf(os.Stderr, "%s\n", err.Error())
      status = 1
    }
  }

  os.Exit(status)
}

func commitFile(path string, info os.FileInfo, force bool) ([]byte, error) {
  var forced bool
  digest, err := repo.HashFile(path, info)
  if err != nil {
    return nil, err
  }

  forced, err = repo.CommitFileHash(path, info, digest, force)
  if forced {
    fmt.Fprintf(os.Stderr, "%s: force write xattrs\n", path)
  }

  return digest, err
}

