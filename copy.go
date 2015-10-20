package main

import (
  "flag"
  "fmt"
  "os"
  "bytes"
  "os/exec"
  "path/filepath"

  repo "github.com/mildred/doc/repo"
  attrs "github.com/mildred/doc/attrs"
  base58 "github.com/jbenet/go-base58"
)

func mainCopy(args []string) {
  f := flag.NewFlagSet("cp", flag.ExitOnError)
  opt_dry_run := f.Bool("n", false, "Dry run")
  opt_force   := f.Bool("f", false, "Force copy even if there are errors")
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
  actions, errors, totalBytes := prepareCopy(src, dst)

  for _, e := range errors {
    status = 1
    fmt.Fprintf(os.Stderr, "%s\n", e.Error())
  }

  var conflicts []string

  var execBytes uint64 = 0
  bytescount := fmt.Sprintf("%d", len(fmt.Sprintf("%d", totalBytes)))

  if len(errors) == 0 || *opt_force || *opt_dry_run {
    for _, act := range actions {
      if act.conflict {
        conflicts = append(conflicts, act.dst)
      }
      if *opt_dry_run {
        fmt.Println(act.show())
      } else {
        err := act.run()
        execBytes += uint64(act.size)
        if err != nil {
          fmt.Fprintf(os.Stderr, "%s\n", err.Error())
          status = 1
          if ! *opt_force {
            break
          }
        } else {
          fmt.Printf("\r\x1b[2K%" + bytescount + "d / %d (%2.0f%%) %s\r", execBytes, totalBytes, 100.0 * float64(execBytes) / float64(totalBytes), act.dst)
        }
      }
    }
  }
  fmt.Println()

  for _, c := range conflicts {
    fmt.Fprintf(os.Stderr, "CONFLICT %s\n", c)
  }

  os.Exit(status)
}

type copyAction struct {
  src string
  dst string
  size int64
  originaldst string
  conflict bool
}

func (act *copyAction) show() string {
  return fmt.Sprintf("cp -a --reflink=auto %s %s\n", act.src, act.dst)
}

func (act *copyAction) run() error {
  err := exec.Command("cp", "-a", "--reflink=auto", act.src, act.dst).Run()
  if err != nil {
    return fmt.Errorf("cp %s: %s", act.dst, err.Error())
  }
  hash, err := attrs.Get(act.src, repo.XattrHash)
  if err != nil {
    return err
  }
  hashTime, err := attrs.Get(act.src, repo.XattrHashTime)
  if err != nil {
    return err
  }
  err = attrs.Set(act.dst, repo.XattrHash, hash)
  if err != nil {
    return err
  }
  err = attrs.Set(act.dst, repo.XattrHashTime, hashTime)
  if err != nil {
    return err
  }
  if act.conflict {
    err = repo.MarkConflictFor(act.dst, filepath.Base(act.originaldst))
    if err != nil {
      return err
    }
    err = repo.AddConflictAlternative(act.originaldst, filepath.Base(act.dst))
    if err != nil {
      return err
    }
  }
  return nil
}

func prepareCopy(src, dst string) (actions []copyAction, errors []error, totalBytes uint64) {
  totalBytes = 0

  srci, err := os.Stat(src)
  if err != nil {
    errors = append(errors, err)
    return
  }

  dsti, err := os.Stat(dst)
  if os.IsNotExist(err) {
    actions = append(actions, copyAction{src, dst, srci.Size(), "", false})
    totalBytes = uint64(srci.Size())
    return
  } else if srci.IsDir() && dsti.IsDir() {
    f, err := os.Open(src)
    if err != nil {
      errors = append(errors, err)
      return
    }
    defer f.Close()
    names, err := f.Readdirnames(-1)
    if err != nil {
      errors = append(errors, err)
      return
    }
    for _, name := range names {
      acts, errs, b := prepareCopy(filepath.Join(src, name), filepath.Join(dst, name))
      if len(errs) > 0 {
        errors = append(errors, errs...)
      }
      if len(acts) > 0 {
        actions = append(actions, acts...)
      }
      totalBytes += b
    }
    return
  } else {
    var srch, dsth []byte
    if ! srci.IsDir() {
      srch, err = repo.GetHash(src, srci)
      if err != nil {
        errors = append(errors, err)
        return
      }
    }
    if ! dsti.IsDir() {
      dsth, err = repo.GetHash(dst, dsti)
      if err != nil {
        errors = append(errors, err)
        return
      }
    }
    if bytes.Equal(srch, dsth) {
      return
    }

    totalBytes = uint64(srci.Size())
    dstname := repo.FindConflictFileName(dst, base58.Encode(srch))
    actions = append(actions, copyAction{src, dstname, srci.Size(), dst, true})
    return
  }
}

func copyEntry(src, dst string, dry_run bool) ([]string, error) {
  srci, err := os.Stat(src)
  if err != nil {
    return nil, err
  }

  dsti, err := os.Stat(dst)
  if os.IsNotExist(err) {
    if dry_run {
      fmt.Printf("cp -a --reflink=auto %s %s\n", src, dst)
    } else {
      err = exec.Command("cp", "-a", "--reflink=auto", src, dst).Run()
      if err != nil {
        return nil, fmt.Errorf("cp %s: %s", dst, err.Error())
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
      fmt.Printf("cp -a --reflink=auto %s %s\n", src, dstname)
    } else {
      err = exec.Command("cp", "-a", "--reflink=auto", src, dstname).Run()
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


