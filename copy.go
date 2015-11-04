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

const copyUsage string =
`doc cp [OPTIONS...] [SRC] DEST

Copy each files in SRC or the current directory over to DEST. Both arguments are
assumed to be directories and cp will synchronize from the source to the
destination in the following way:

 *  Files from source not in the destination: the file is copied
 
 *  Files from source existing in the destination with identical content: no
    action is needed
 
 *  Files from source existing in the destination with different content: the
    file is copied under a new name, and a conflict is registred with the
    original file in the destination directory.

Options:
`

func mainCopy(args []string) {
  f := flag.NewFlagSet("cp", flag.ExitOnError)
  opt_dry_run := f.Bool("n", false, "Dry run")
  opt_force   := f.Bool("f", false, "Force copy even if there are errors")
  f.Usage = func(){
    fmt.Print(copyUsage)
    f.PrintDefaults()
  }
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

  actions, errors, totalBytes := prepareCopy(src, dst)

  for _, e := range errors {
    fmt.Fprintf(os.Stderr, "%s\n", e.Error())
  }

  var conflicts []string
  var nerrors int

  if len(errors) == 0 || *opt_force || *opt_dry_run {
    conflicts, nerrors = performActions(actions, totalBytes, *opt_dry_run, *opt_force)
    nerrors = nerrors + len(errors)
  }

  for _, c := range conflicts {
    fmt.Fprintf(os.Stderr, "CONFLICT %s\n", c)
  }

  if nerrors > 0 {
   os.Exit(1)
  }
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

    //
    // File in source but not in destination
    //

    actions = append(actions, copyAction{src, dst, srci.Size(), "", false})
    totalBytes = uint64(srci.Size())
    return

  } else if srci.IsDir() && dsti.IsDir() {

    //
    // Both source and destination are directories, merge
    //

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

    //
    // Source and destination are regular files
    // If hash is different, there is a conflict
    //

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

func performActions(actions []copyAction, totalBytes uint64, dry_run, force bool) (conflicts []string, nerrors int) {
  var execBytes uint64 = 0
  bytescount := fmt.Sprintf("%d", len(fmt.Sprintf("%d", totalBytes)))

  for _, act := range actions {
    if act.conflict {
      conflicts = append(conflicts, act.dst)
    }
    if dry_run {
      fmt.Println(act.show())
    } else {
      err := act.run()
      execBytes += uint64(act.size)
      if err != nil {
        fmt.Fprintf(os.Stderr, "%s\n", err.Error())
        nerrors = nerrors + 1
        if ! force {
          break
        }
      } else {
        fmt.Printf("\r\x1b[2K%" + bytescount + "d / %d (%2.0f%%) %s\r", execBytes, totalBytes, 100.0 * float64(execBytes) / float64(totalBytes), act.dst)
      }
    }
  }
  fmt.Println()

  return
}

