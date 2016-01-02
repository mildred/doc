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

Show missing files in SRC that are in DEST

Options:
`

func mainMissing(args []string) int {
  f := flag.NewFlagSet("status", flag.ExitOnError)
  opt_from    := f.String("from", "", "Specify the source directory")
  opt_to      := f.String("to", "", "Specify the destination directory")
  f.Usage = func(){
    fmt.Print(usageStatus)
    f.PrintDefaults()
  }
  f.Parse(args)
  src, dst := findSourceDest(*opt_from, *opt_to, f.Args())

  srccommit := filepath.Join(src, ".doccommit")
  dstcommit := filepath.Join(dst, ".doccommit")

  srcfiles, err := commit.Read(srccommit)
  if err != nil {
    fmt.Fprintf(os.Stderr, "%s: %s", srccommit, err.Error())
  }

  dstfiles, err := commit.Read(dstcommit)
  if err != nil {
    fmt.Fprintf(os.Stderr, "%s: %s", dstcommit, err.Error())
  }

  for hash, dfiles := range dstfiles {
    sfiles := srcfiles[hash]
    for _, sfile := range sfiles {
      same_file := false
      for _, dfile := range dfiles {
        if dfile == sfile {
          same_file = true
          break
        }
        if ! same_file {
          fmt.Printf("%s\t%s\n", hash, commit.EncodePath(dfile))
        }
      }
    }
  }

  return 0
}


