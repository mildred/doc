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

const syncUsage string =
`doc sync [OPTIONS...] [SRC] DEST
doc sync [OPTIONS...] -from SRC [DEST]
doc sync [OPTIONS...] -to DEST [SRC]

Copy each files in SRC or the current directory over to DEST, and each of DEST
over to SRC. Both arguments are assumed to be directories and the
synchronisation will be according to the following rules:

 *  Files from source not in the destination: the file is copied
 
 *  Files not in source but in destination: the file is copied
 
 *  Files from source existing in the destination with identical content: no
    action is needed
 
 *  Files from source existing in the destination with different content: the
    file is copied under a new name in both directions (the original files are
    kept) and a conflict is registered with the original files.

Options:
`

const copyUsage string =
`doc cp [OPTIONS...] [SRC] DEST
doc cp [OPTIONS...] -from SRC [DEST]
doc cp [OPTIONS...] -to DEST [SRC]

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
  opt_from    := f.String("from", "", "Specify the source directory")
  opt_to      := f.String("to", "", "Specify the destination directory")
  f.Usage = func(){
    fmt.Print(copyUsage)
    f.PrintDefaults()
  }
  f.Parse(args)

  src, dst := findSourceDest(*opt_from, *opt_to, f.Args())
  syncOrCopy(src, dst, *opt_dry_run, *opt_force, false)
}

func mainSync(args []string) {
  f := flag.NewFlagSet("sync", flag.ExitOnError)
  opt_dry_run := f.Bool("n", false, "Dry run")
  opt_force   := f.Bool("f", false, "Force copy even if there are errors")
  opt_from    := f.String("from", "", "Specify the source directory")
  opt_to      := f.String("to", "", "Specify the destination directory")
  f.Usage = func(){
    fmt.Print(syncUsage)
    f.PrintDefaults()
  }
  f.Parse(args)

  src, dst := findSourceDest(*opt_from, *opt_to, f.Args())
  syncOrCopy(src, dst, *opt_dry_run, *opt_force, true)
}

func findSourceDest(opt_src, opt_dst string, args []string) (src string, dst string) {
  src = opt_src
  dst = opt_dst
  if src == "" && dst == "" {
    src = args[0]
    dst = args[1]
    if dst == "" {
      dst = src
      src = "."
    }
  } else if src == "" {
    dst = args[0]
    if dst == "" {
      dst = "."
    }
  } else if dst == "" {
    src = args[0]
    if src == "" {
      src = "."
    }
  }

  if src == "" || dst == "" {
    fmt.Fprintln(os.Stderr, "You must specify at least the destination directory")
    os.Exit(1)
  }

  return
}

func syncOrCopy(src, dst string, dry_run, force, bidir bool){
  actions, errors, totalBytes := prepareCopy(src, dst, true)

  for _, e := range errors {
    fmt.Fprintf(os.Stderr, "%s\n", e.Error())
  }

  var conflicts []string
  var nerrors int

  if len(errors) == 0 || force || dry_run {
    conflicts, nerrors = performActions(actions, totalBytes, dry_run, force)
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

func prepareCopy(src, dst string, bidir bool) (actions []copyAction, errors []error, totalBytes uint64) {
  var err error
  totalBytes = 0

  srci, srcerr := os.Stat(src)
  dsti, dsterr := os.Stat(dst)

  //
  // File in source but not in destination
  //

  if os.IsNotExist(dsterr) && srcerr == nil {

    actions = append(actions, copyAction{src, dst, srci.Size(), "", false})
    totalBytes = uint64(srci.Size())
    return

  }

  //
  // [bidir] File in destination but not in source
  //

  if bidir && os.IsNotExist(srcerr) && dsterr == nil {
    actions = append(actions, copyAction{dst, src, dsti.Size(), "", false})
    totalBytes = uint64(dsti.Size())
    return
  }

  if srcerr != nil {
    errors = append(errors, srcerr)
    return
  }

  if dsterr != nil {
    errors = append(errors, dsterr)
    return
  }

  //
  // Both source and destination are directories, merge
  //

  if srci.IsDir() && dsti.IsDir() {

    var srcnames map[string]bool

    if bidir {
      srcnames = map[string]bool{}
    }

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
      if bidir {
        srcnames[name] = true
      }
      acts, errs, b := prepareCopy(filepath.Join(src, name), filepath.Join(dst, name), bidir)
      if len(errs) > 0 {
        errors = append(errors, errs...)
      }
      if len(acts) > 0 {
        actions = append(actions, acts...)
      }
      totalBytes += b
    }

    if bidir {

      f, err := os.Open(dst)
      if err != nil {
        errors = append(errors, err)
        return
      }
      defer f.Close()
      dstnames, err := f.Readdirnames(-1)
      if err != nil {
        errors = append(errors, err)
        return
      }

      for _, name := range dstnames {
        if srcnames[name] {
          continue
        }
        acts, errs, b := prepareCopy(filepath.Join(src, name), filepath.Join(dst, name), bidir)
        if len(errs) > 0 {
          errors = append(errors, errs...)
        }
        if len(acts) > 0 {
          actions = append(actions, acts...)
        }
        totalBytes += b
      }

    }

    return

  }

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

  totalBytes = 0

  if repo.ConflictFile(src) == "" {
    totalBytes += uint64(srci.Size())
    dstname := repo.FindConflictFileName(dst, base58.Encode(srch))
    actions = append(actions, copyAction{src, dstname, srci.Size(), dst, true})
  }

  if bidir && repo.ConflictFile(dst) == "" {
    totalBytes += uint64(dsti.Size())
    srcname := repo.FindConflictFileName(src, base58.Encode(dsth))
    actions = append(actions, copyAction{dst, srcname, dsti.Size(), src, true})
  }

  return
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

