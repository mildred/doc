package main

import (
  "flag"
  "fmt"
  "os"
  "bytes"
  "os/exec"
  "path/filepath"

  repo "github.com/mildred/doc/repo"
  base58 "github.com/jbenet/go-base58"
)

func mainCopy(args []string) {
  f := flag.NewFlagSet("cp", flag.ExitOnError)
  opt_dry_run := f.Bool("n", false, "Dry run")
  f.Parse(args)
  src := f.Arg(0)
  dst := f.Arg(1)

  if src == "" && dst == "" {
    fmt.Fprintln(os.Stderr, "You must specify at least the destination directory")
    os.Exit(1)
  } else if dst == "" {
    dst = src
    src = "."
  }

  status := 0
  conflicts, err := copyEntry(src, dst, *opt_dry_run)

  for _, c := range conflicts {
    fmt.Fprintf(os.Stderr, "CONFLICT %s\n", c)
  }

  if err != nil {
    fmt.Fprintf(os.Stderr, "%v", err)
    os.Exit(1)
  }

  os.Exit(status)
}

func copyEntry(src, dst string, dry_run bool) ([]string, error) {
  srci, err := os.Stat(src)
  if err != nil {
    return nil, err
  }

  dsti, err := os.Stat(dst)
  if os.IsNotExist(err) {
    if dry_run {
      fmt.Printf("cp -la %s %s\n", src, dst)
    } else {
      err = exec.Command("cp", "-la", src, dst).Run()
      if err != nil {
        return nil, err
      }
    }
    return nil, nil
  } else if srci.IsDir() && dsti.IsDir() {
    conflicts := []string{}
    f, err := os.Open(src)
    if err != nil {
      return nil, err
    }
    defer f.Close()
    names, err := f.Readdirnames(-1)
    if err != nil {
      return nil, err
    }
    for _, name := range names {
      c, err := copyEntry(filepath.Join(src, name), filepath.Join(dst, name), dry_run)
      if err != nil {
        fmt.Fprintf(os.Stderr, "%s\n", err.Error())
      } else {
        conflicts = append(conflicts, c...)
      }
    }
    return conflicts, nil
  } else {
    var srch, dsth []byte
    if ! srci.IsDir() {
      srch, err = repo.GetHash(src, srci)
      if err != nil {
        return nil, err
      }
    }
    if ! dsti.IsDir() {
      dsth, err = repo.GetHash(dst, dsti)
      if err != nil {
        return nil, err
      }
    }
    if bytes.Equal(srch, dsth) {
      return nil, nil
    }

    dstname := repo.FindConflictFileName(dst, base58.Encode(srch))
    if dry_run {
      fmt.Printf("cp -la %s %s\n", src, dstname)
    } else {
      err = exec.Command("cp", "-la", src, dstname).Run()
      if err != nil {
        return nil, fmt.Errorf("cp %s: %s", dstname, err.Error())
      }
      err = repo.MarkConflictFor(dstname, filepath.Base(dst))
      if err != nil {
        return nil, err
      }
      err = repo.AddConflictAlternative(dst, filepath.Base(dstname))
      if err != nil {
        return nil, err
      }
    }
    return []string{dstname}, nil
  }
}


