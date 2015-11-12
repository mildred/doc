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

  *     Files from source not in the destination: the file is copied
 
  *     Files not in source but in destination: the file is copied
 
  *     Files from source existing in the destination with identical content: no
        action is needed
 
  *     Files from source existing in the destination with different content:
        the file is copied under a new name in both directions (the original
        files are kept) and a conflict is registered with the original files.

Options:
`

const copyUsage string =
`doc cp [OPTIONS...] [SRC] DEST
doc cp [OPTIONS...] -from SRC [DEST]
doc cp [OPTIONS...] -to DEST [SRC]

Copy each files in SRC or the current directory over to DEST. Both arguments are
assumed to be directories and cp will synchronize from the source to the
destination in the following way:

  *     Files from source not in the destination: the file is copied
 
  *     Files from source existing in the destination with identical content: no
        action is needed
 
  *     Files from source existing in the destination with different content:
        the file is copied under a new name, and a conflict is registred with
        the original file in the destination directory.

Options:
`

func mainCopy(args []string) {
  f := flag.NewFlagSet("cp", flag.ExitOnError)
  opt_dry_run := f.Bool("n", false, "Dry run")
  opt_quiet   := f.Bool("q", false, "Quiet")
  opt_force   := f.Bool("f", false, "Force copy even if there are errors")
  opt_hash    := f.Bool("c", false, "If necessary, check real hash when deduplicating")
  opt_dedup   := f.Bool("d", false, "Deduplicate files on destination (link files on destination instead of copying them from source if possible)")
  opt_dd      := f.Bool("dd", false, "Like -d but also remove duplicate files")
  opt_from    := f.String("from", "", "Specify the source directory")
  opt_to      := f.String("to", "", "Specify the destination directory")
  f.Usage = func(){
    fmt.Print(copyUsage)
    f.PrintDefaults()
  }
  f.Parse(args)

  src, dst := findSourceDest(*opt_from, *opt_to, f.Args())
  syncOrCopy(src, dst, *opt_dry_run, *opt_force, *opt_quiet, *opt_dedup || *opt_dd, *opt_dd, *opt_hash, false)
}

func mainSync(args []string) {
  f := flag.NewFlagSet("sync", flag.ExitOnError)
  opt_dry_run := f.Bool("n", false, "Dry run")
  opt_quiet   := f.Bool("q", false, "Quiet")
  opt_force   := f.Bool("f", false, "Force copy even if there are errors")
  opt_from    := f.String("from", "", "Specify the source directory")
  opt_to      := f.String("to", "", "Specify the destination directory")
  f.Usage = func(){
    fmt.Print(syncUsage)
    f.PrintDefaults()
  }
  f.Parse(args)

  src, dst := findSourceDest(*opt_from, *opt_to, f.Args())
  syncOrCopy(src, dst, *opt_dry_run, *opt_force, *opt_quiet, false, false, false, true)
}

func findSourceDest(opt_src, opt_dst string, args []string) (src string, dst string) {
  var arg0, arg1 string
  if len(args) > 0 {
    arg0 = args[0]
  }
  if len(args) > 1 {
    arg1 = args[1]
  }
  src = opt_src
  dst = opt_dst
  if src == "" && dst == "" {
    src = arg0
    dst = arg1
    if dst == "" {
      dst = src
      src = "."
    }
  } else if src == "" {
    dst = arg0
    if dst == "" {
      dst = "."
    }
  } else if dst == "" {
    src = arg0
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

func syncOrCopy(src, dst string, dry_run, force, quiet, dedup, delete_dup, check_hash, bidir bool){
  var dedup_map map[string][]string = nil
  if dedup {
    dedup_map = map[string][]string{}
  }

  actions, errors, totalBytes := prepareCopy(src, dst, check_hash, bidir, dedup_map)

  for _, e := range errors {
    fmt.Fprintf(os.Stderr, "%s\n", e.Error())
  }

  var conflicts []string
  var nerrors int
  var dup_hashes [][]byte

  if len(errors) == 0 || force || dry_run {
    conflicts, nerrors, dup_hashes = performActions(actions, totalBytes, dry_run, force, quiet, dedup_map)
    nerrors = nerrors + len(errors)
  }

  if delete_dup {
    for _, h := range dup_hashes {
      for _, path := range dedup_map[string(h)] {
        if dry_run {
          fmt.Sprintf("rm -f %s\n", path)
        } else {
          err := os.Remove(path)
          if err != nil {
            fmt.Fprintf(os.Stderr, "remove %s: %s", path, err.Error())
            nerrors += 1
          }
        }
      }
    }
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
  dsthash []byte
  size int64
  originaldst string
  conflict bool
  link bool
}

func (act *copyAction) show() string {
  if act.link {
    return fmt.Sprintf("ln %s %s\n", act.src, act.dst)
  } else {
    return fmt.Sprintf("cp -a --reflink=auto %s %s\n", act.src, act.dst)
  }
}

func (act *copyAction) run() error {
  var err error
  if act.link {
    err = os.Link(act.src, act.dst)
    if err != nil {
      return fmt.Errorf("link %s: %s", act.dst, err.Error())
    }
  } else {
    err = exec.Command("cp", "-a", "--reflink=auto", act.src, act.dst).Run()
    if err != nil {
      return fmt.Errorf("cp %s: %s", act.dst, err.Error())
    }
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

func prepareCopy(src, dst string, check_hash, bidir bool, dedup map[string][]string) (actions []copyAction, errors []error, totalBytes uint64) {
  var err error
  totalBytes = 0

  srci, srcerr := os.Stat(src)
  dsti, dsterr := os.Stat(dst)

  //
  // File in source but not in destination
  //

  if os.IsNotExist(dsterr) && srcerr == nil {

    srchash, err := repo.GetHash(src, srci, dedup != nil)
    if err != nil {
      errors = append(errors, err)
      return
    }

    actions = append(actions, copyAction{src, dst, srchash, srci.Size(), "", false, false})
    totalBytes = uint64(srci.Size())
    return

  }

  //
  // [bidir] File in destination but not in source
  //

  if (bidir || dedup != nil) && os.IsNotExist(srcerr) && dsterr == nil {

    // Synchronize in the other direction
    if bidir {
      dsthash, err := repo.GetHash(dst, dsti, dedup != nil)
      if err != nil {
        errors = append(errors, err)
        return
      }

      actions = append(actions, copyAction{dst, src, dsthash, dsti.Size(), "", false, false})
      totalBytes = uint64(dsti.Size())
      return
    }

    // Record dst hash in case we move it
    if dedup != nil {
      hash, err := repo.GetHash(dst, dsti, check_hash)
      if err != nil {
        errors = append(errors, err)
      } else {
        dedup[string(hash)] = append(dedup[string(hash)], dst)
      }
    }
  }

  //
  // Handle stat() errors
  //

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
      acts, errs, b := prepareCopy(filepath.Join(src, name), filepath.Join(dst, name), check_hash, bidir, dedup)
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
        acts, errs, b := prepareCopy(filepath.Join(src, name), filepath.Join(dst, name), check_hash, bidir, dedup)
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
    srch, err = repo.GetHash(src, srci, true)
    if err != nil {
      errors = append(errors, err)
      return
    }
  }
  if ! dsti.IsDir() {
    dsth, err = repo.GetHash(dst, dsti, true)
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
    actions = append(actions, copyAction{src, dstname, nil, srci.Size(), dst, true, false})
  }

  if bidir && repo.ConflictFile(dst) == "" {
    totalBytes += uint64(dsti.Size())
    srcname := repo.FindConflictFileName(src, base58.Encode(dsth))
    actions = append(actions, copyAction{dst, srcname, nil, dsti.Size(), src, true, false})
  }

  return
}

func performActions(actions []copyAction, totalBytes uint64, dry_run, force, quiet bool, dedup map[string][]string) (conflicts []string, nerrors int, duplicate_hashes [][]byte) {
  var execBytes uint64 = 0
  bytescount := fmt.Sprintf("%d", len(fmt.Sprintf("%d", totalBytes)))

  for _, act := range actions {
    if act.conflict {
      conflicts = append(conflicts, act.dst)
    }
    if dedup != nil && act.dsthash != nil {
      if files, ok := dedup[string(act.dsthash)]; ok && len(files) > 0 {
        duplicate_hashes = append(duplicate_hashes, act.dsthash)
        act.src = files[0]
        act.link = true
      }
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
      } else if ! quiet {
        fmt.Printf("\r\x1b[2K%" + bytescount + "d / %d (%2.0f%%) %s\r", execBytes, totalBytes, 100.0 * float64(execBytes) / float64(totalBytes), act.dst)
      }
    }
  }
  if ! quiet {
    fmt.Println()
  }

  return
}

