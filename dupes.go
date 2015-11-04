package main

import (
  "os"
  "fmt"
  "flag"
  "syscall"

  repo "github.com/mildred/doc/repo"
  base58 "github.com/jbenet/go-base58"
)

const dupesUsage string =
`doc dupes [DIR...]

List duplicate files under the given directories (or the current directory if
none given). Group of identical files are separated by a blank line. The number
of detected links is shown in front of each file. Modified files are not listed.

WARNING: deduplication of identical files do not currently check that the files
are exactly the same. it just checks that the recorded hash are identical. The
file might have been modified since. You should run doc check on the duplicate
files before you try to deduplicate them.

Options:
`

type sameFile struct {
  hash    []byte
  paths   []string
  inodes  []uint64
  devices []uint64
}

func mainDupes(args []string) {
  f := flag.NewFlagSet("dupes", flag.ExitOnError)
  opt_show_links := f.Bool("l", false, "Show group of files that share the same inode")
  opt_progress := f.Bool("p", false, "Show progress")
  opt_dedup := f.Bool("d", false, "Deduplicate files (make links)")
  f.Usage = func(){
    fmt.Print(dupesUsage)
    f.PrintDefaults()
  }
  f.Parse(args)
  srcs := f.Args()

  if len(srcs) == 0 {
    srcs = append(srcs, ".")
  }

  dupes := map[string]sameFile{}

  num := 0
  errors := 0

  for _, src := range srcs {
    e := repo.Walk(src, func(path string, info os.FileInfo)error{

      mtime, err := repo.GetHashTime(path)
      if err != nil {
        return err
      }

      // check that the file is up to date
      if mtime != info.ModTime() {
        return nil
      }

      hash, err := repo.GetHash(path, info)
      if err != nil {
        return err
      }

      sys, ok := info.Sys().(*syscall.Stat_t)
      if ! ok {
        sys.Ino = 0
      }

      f := dupes[string(hash)]
      f.hash = hash
      f.paths = append(f.paths, path)
      f.inodes = append(f.inodes, sys.Ino)
      f.devices = append(f.devices, sys.Dev)
      dupes[string(hash)] = f

      num = num + 1
      if *opt_progress {
        fmt.Printf("\r\x1b[2K%d %s\r", num, path)
      }

      return nil
    }, func(path string, info os.FileInfo, err error)bool{
      fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
      return true
    })
    errors = errors + len(e)
  }

  for _, f := range dupes {
    if len(f.paths) <= 1 {
      continue
    }
    files := map[uint64][]string{}
    for i, ino := range f.inodes {
      files[ino] = append(files[ino], f.paths[i])
    }
    if len(files) == 1 && !*opt_show_links {
      continue
    }
    fmt.Println()
    hash := base58.Encode(f.hash)
    for _, paths := range files {
      for _, path := range paths {
        fmt.Printf("%s\t%d\t%s\n", hash, len(paths), path)
      }
    }
    if len(files) > 1 && *opt_dedup {
      err := deduplicate(f)
      if err != nil {
        fmt.Fprintf(os.Stderr, "%s", err.Error())
        errors = errors + 1
      }
    }
  }

  if errors > 0 {
    os.Exit(1)
  }
}

func deduplicate(f sameFile) error {
  by_dev := map[uint64][]int{}
  for i, dev := range f.devices {
    by_dev[dev] = append(by_dev[dev], i)
  }
  for _, file_list := range by_dev {
    if len(file_list) <= 1 {
      continue
    }
    first_file := file_list[0]
    for _, cur_file := range file_list[1:] {
      if f.inodes[first_file] == f.inodes[cur_file] {
        continue
      }
      err := os.Remove(f.paths[cur_file])
      if err != nil {
        return err
      }
      err = os.Link(f.paths[first_file], f.paths[cur_file])
      if err != nil {
        panic(fmt.Errorf("Could not link identical file '%s' to '%s': %s", f.paths[first_file], f.paths[cur_file], err.Error()))
      }
    }
  }
  return nil
}

