package sync

import (
  "os"
  "fmt"
  "bytes"
  "path/filepath"
  "github.com/mildred/doc/repo"
  base58 "github.com/jbenet/go-base58"
)

type Preparator struct {
  // If true, scan files that are out of date to get the new hash
  CheckHash bool

  // Bidirectional scan: return actions to synchronize both from source to
  // destination and from destination to source.
  Bidir bool

  // If true, commit new hash for out of date files
  Commit bool

  // Gather dediplication data if not nil: for each hash (as keys, binary hash:
  // not digests), store a list of matching paths (for the destination directory
  // only).
  Dedup map[string][]string

  Actions []CopyAction
  Errors []error

  // Logger to be called for each scanned item
  // Always called with hashing to false. When performing hashing, it is called
  // a second or a third time with either hash_src or hash_dst set to true,
  // depending on which file is being hashed.
  Logger func(p *Preparator, src, dst string, hash_src, hash_dst bool)

  // Total bytes scanned that can be counted (excluding directories)
  TotalBytes uint64

  // Total items scanned
  NumFiles uint64

  // for Log function
  hashingMsg bool
}

func Log(p *Preparator, src, dst string, hash_src, hash_dst bool) {
  if (hash_src || hash_dst) && !p.hashingMsg {
    fmt.Printf(" (hashing)\r");
    p.hashingMsg = true
  } else {
    fmt.Printf("\r\x1b[2K%6d files scanned, %9d bytes to copy: scanning %s", p.NumFiles, p.TotalBytes, filepath.Base(src));
    p.hashingMsg = false
  }
}

func (p *Preparator) PrepareCopy(src, dst string) {
  var err error

  if p.Logger != nil {
    p.Logger(p, src, dst, false, false)
  }
  p.NumFiles += 1

  srci, srcerr := os.Stat(src)
  dsti, dsterr := os.Stat(dst)

  //
  // File in source but not in destination
  //

  if os.IsNotExist(dsterr) && srcerr == nil {

    srchash, err := repo.GetHash(src, srci, p.Dedup != nil)
    if err != nil {
      p.Errors = append(p.Errors, err)
      return
    }

    p.Actions = append(p.Actions, *NewCopyAction(src, dst, srchash, srci.Size(), "", false, false))
    p.TotalBytes += uint64(srci.Size())
    return

  }

  //
  // [Bidir] File in destination but not in source
  //

  if (p.Bidir || p.Dedup != nil) && os.IsNotExist(srcerr) && dsterr == nil {

    // Synchronize in the other direction
    if p.Bidir {
      dsthash, err := repo.GetHash(dst, dsti, p.Dedup != nil)
      if err != nil {
        p.Errors = append(p.Errors, err)
        return
      }

      p.Actions = append(p.Actions, *NewCopyAction(dst, src, dsthash, dsti.Size(), "", false, false))
      p.TotalBytes += uint64(dsti.Size())
      return
    }

    // Record dst hash in case we move it
    if p.Dedup != nil {
      hash, err := repo.GetHash(dst, dsti, p.CheckHash)
      if err != nil {
        p.Errors = append(p.Errors, err)
      } else {
        p.Dedup[string(hash)] = append(p.Dedup[string(hash)], dst)
      }
    }
  }

  //
  // Handle stat() errors
  //

  if srcerr != nil {
    p.Errors = append(p.Errors, srcerr)
    return
  }

  if dsterr != nil {
    p.Errors = append(p.Errors, dsterr)
    return
  }

  //
  // Both source and destination are directories, merge
  //

  if srci.IsDir() && dsti.IsDir() {

    var srcnames map[string]bool

    if p.Bidir {
      srcnames = map[string]bool{}
    }

    f, err := os.Open(src)
    if err != nil {
      p.Errors = append(p.Errors, err)
      return
    }
    defer f.Close()
    names, err := f.Readdirnames(-1)
    if err != nil {
      p.Errors = append(p.Errors, err)
      return
    }

    for _, name := range names {
      if p.Bidir {
        srcnames[name] = true
      }
      p.PrepareCopy(filepath.Join(src, name), filepath.Join(dst, name))
    }

    if p.Bidir {

      f, err := os.Open(dst)
      if err != nil {
        p.Errors = append(p.Errors, err)
        return
      }
      defer f.Close()
      dstnames, err := f.Readdirnames(-1)
      if err != nil {
        p.Errors = append(p.Errors, err)
        return
      }

      for _, name := range dstnames {
        if srcnames[name] {
          continue
        }
        p.PrepareCopy(filepath.Join(src, name), filepath.Join(dst, name))
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
    srch, err = repo.GetHash(src, srci, false)
    computed := false
    if err == nil && srch == nil {
      if p.Logger != nil {
        p.Logger(p, src, dst, true, false)
      }
      srch, err = repo.HashFile(src)
      computed = true
    }
    if err == nil && computed && p.Commit {
      _, err = repo.CommitFileHash(src, srci, srch, false)
    }
    if err != nil {
      p.Errors = append(p.Errors, err)
      return
    }
  }
  if ! dsti.IsDir() {
    dsth, err = repo.GetHash(dst, dsti, false)
    computed := false
    if err == nil && dsth == nil {
      if p.Logger != nil {
        p.Logger(p, src, dst, false, true)
      }
      dsth, err = repo.HashFile(dst)
      computed = true
    }
    if err == nil && computed && p.Commit {
      _, err = repo.CommitFileHash(dst, dsti, dsth, false)
    }
    if err != nil {
      p.Errors = append(p.Errors, err)
      return
    }
  }
  if bytes.Equal(srch, dsth) {
    return
  }

  if repo.ConflictFile(src) == "" {
    p.TotalBytes += uint64(srci.Size())
    dstname := repo.FindConflictFileName(dst, base58.Encode(srch))
    p.Actions = append(p.Actions, *NewCopyAction(src, dstname, nil, srci.Size(), dst, true, false))
  }

  if p.Bidir && repo.ConflictFile(dst) == "" {
    p.TotalBytes += uint64(dsti.Size())
    srcname := repo.FindConflictFileName(src, base58.Encode(dsth))
    p.Actions = append(p.Actions, *NewCopyAction(dst, srcname, nil, dsti.Size(), src, true, false))
  }

  return
}
