package main

import (
  "flag"
  "fmt"
  "os"

  sync "github.com/mildred/doc/sync"
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

Unless the force flag is specified, the operation will stop on the first error.

The operatios is performed in two steps. The first step collects information
about each file and deduce the action to perform, and the second step performs
the actual copy. Interrupting the process during its first step leave your
filesystem untouched. During this parsing step, if files are out of date, their
hash will be computed and that can introduce a delay.

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

Unless the force flag is specified, the operation will stop on the first error.

The operatios is performed in two steps. The first step collects information
about each file and deduce the action to perform, and the second step performs
the actual copy. Interrupting the process during its first step leave your
filesystem untouched. During this parsing step, if files are out of date, their
hash will be computed and that can introduce a delay.

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
  opt_commit  := f.Bool("commit", false, "Commit the new hash if it has been computed")
  f.Usage = func(){
    fmt.Print(copyUsage)
    f.PrintDefaults()
  }
  f.Parse(args)

  src, dst := findSourceDest(*opt_from, *opt_to, f.Args())
  syncOrCopy(src, dst, *opt_dry_run, *opt_force, *opt_quiet, *opt_commit, *opt_dedup || *opt_dd, *opt_dd, *opt_hash, false)
}

func mainSync(args []string) {
  f := flag.NewFlagSet("sync", flag.ExitOnError)
  opt_dry_run := f.Bool("n", false, "Dry run")
  opt_quiet   := f.Bool("q", false, "Quiet")
  opt_force   := f.Bool("f", false, "Force copy even if there are errors")
  opt_from    := f.String("from", "", "Specify the source directory")
  opt_to      := f.String("to", "", "Specify the destination directory")
  opt_commit  := f.Bool("commit", false, "Commit the new hash if it has been computed")
  f.Usage = func(){
    fmt.Print(syncUsage)
    f.PrintDefaults()
  }
  f.Parse(args)

  src, dst := findSourceDest(*opt_from, *opt_to, f.Args())
  syncOrCopy(src, dst, *opt_dry_run, *opt_force, *opt_quiet, *opt_commit, false, false, false, true)
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
  } else if dst == "" {
    dst = arg0
    if dst == "" {
      dst = "."
    }
  } else if src == "" {
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

func syncOrCopy(src, dst string, dry_run, force, quiet, commit, dedup, delete_dup, check_hash, bidir bool){
  var dedup_map map[string][]string = nil
  if dedup {
    dedup_map = map[string][]string{}
  }

  if ! quiet {
    fmt.Printf("Source:      %s\n", src)
    fmt.Printf("Destination: %s\n", dst)
    fmt.Printf("Step 1: Prepare copy...\n")
  }

  prep := &sync.Preparator{
    CheckHash: check_hash,
    Bidir: bidir,
    Quiet: quiet,
    Commit: commit,
    Dedup: dedup_map,
  }

  prep.PrepareCopy(src, dst)

  if ! quiet {
    fmt.Printf("\x1b[2K")
  }

  for _, e := range prep.Errors {
    fmt.Fprintf(os.Stderr, "%s\n", e.Error())
  }

  var conflicts []string
  var nerrors int
  var dup_hashes [][]byte

  if ! quiet {
    fmt.Printf("Step 2: Copy files...\n")
  }

  if len(prep.Errors) == 0 || force || dry_run {
    if ! quiet && len(prep.Errors) > 0 {
      fmt.Printf("Errors found but continuing operation\n");
    }
    conflicts, nerrors, dup_hashes = performActions(prep.Actions, prep.TotalBytes, dry_run, force, quiet, dedup_map)
    nerrors = nerrors + len(prep.Errors)
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

func performActions(actions []sync.CopyAction, totalBytes uint64, dry_run, force, quiet bool, dedup map[string][]string) (conflicts []string, nerrors int, duplicate_hashes [][]byte) {
  var execBytes uint64 = 0
  bytescount := fmt.Sprintf("%d", len(fmt.Sprintf("%d", totalBytes)))

  for _, act := range actions {
    if act.Conflict {
      conflicts = append(conflicts, act.Dst)
    }
    if dedup != nil && act.Dsthash != nil {
      if files, ok := dedup[string(act.Dsthash)]; ok && len(files) > 0 {
        duplicate_hashes = append(duplicate_hashes, act.Dsthash)
        act.Src = files[0]
        act.Link = true
      }
    }
    if dry_run {
      fmt.Println(act.Show())
    } else {
      err := act.Run()
      execBytes += uint64(act.Size)
      if err != nil {
        fmt.Fprintf(os.Stderr, "%s\n", err.Error())
        nerrors = nerrors + 1
        if ! force {
          break
        }
      } else if ! quiet {
        fmt.Printf("\r\x1b[2K%" + bytescount + "d / %d (%2.0f%%) %s\r", execBytes, totalBytes, 100.0 * float64(execBytes) / float64(totalBytes), act.Dst)
      }
    }
  }
  if ! quiet {
    fmt.Println()
  }

  return
}

