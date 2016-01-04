package main

import (
  "flag"
  "os"
  "fmt"
  "path/filepath"
  commit "github.com/mildred/doc/commit"
)

const usageMissing string =
`doc missing [OPTIONS...] [SRC] DEST
doc missing [OPTIONS...] -from SRC [DEST]
doc missing [OPTIONS...] -to DEST [SRC]

Show files in SRC that are different from the same files in DEST. Lines start
with a symbol, followed by the file hash and the file name.

  -     Represents the file in SRC
  +     Represents the file in DEST

Conflicts are represented by two lines (one for each version).

Options:
`

func mainMissing(args []string) int {
  f := flag.NewFlagSet("status", flag.ExitOnError)
  opt_from    := f.String("from", "", "Specify the source directory")
  opt_to      := f.String("to", "", "Specify the destination directory")
  f.Usage = func(){
    fmt.Print(usageMissing)
    f.PrintDefaults()
  }
  f.Parse(args)
  src, dst := findSourceDest(*opt_from, *opt_to, f.Args())

  srccommit := filepath.Join(src, ".doccommit")
  dstcommit := filepath.Join(dst, ".doccommit")

  srcfiles, err := commit.ReadByPath(srccommit)
  if err != nil {
    fmt.Fprintf(os.Stderr, "%s: %s", srccommit, err.Error())
  }

  dstfiles, err := commit.ReadByPath(dstcommit)
  if err != nil {
    fmt.Fprintf(os.Stderr, "%s: %s", dstcommit, err.Error())
  }

  for file, shash := range srcfiles {
    dhash := dstfiles[file]
    if shash != dhash {
      fmt.Printf("- %s\t%s\n", shash, commit.EncodePath(file))
      if dhash != "" {
        fmt.Printf("+ %s\t%s\n", shash, commit.EncodePath(file))
      }
    }
  }

  return 0
}


