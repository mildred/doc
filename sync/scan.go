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

  // [MANDATORY] Called when an action is to be taken. Should return true. False
  // is used to stop scanning
  HandleAction func(act CopyAction) bool

  // [MANDATORY] Called when an error happens. Should return true to continue
  // scanning or false to stop scaning.
  HandleError func (e error) bool

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

func (p *Preparator) PrepareCopy(src, dst string) bool {
  var err error

  if p.Logger != nil {
    p.Logger(p, src, dst, false, false)
  }
  p.NumFiles += 1

  srci, srcerr := os.Lstat(src)
  dsti, dsterr := os.Lstat(dst)

  srcsymlink := srci != nil && srci.Mode() & os.ModeSymlink != 0
  dstsymlink := dsti != nil && dsti.Mode() & os.ModeSymlink != 0

  //
  // File in source but not in destination
  //

  if os.IsNotExist(dsterr) && srcerr == nil {

    var srchash []byte

    if ! srci.IsDir() {
      srchash, err = repo.GetHash(src, srci, p.Dedup != nil)
      if err != nil {
        return p.HandleError(err)
      }
    }

    res := p.HandleAction(*NewCopyAction(src, dst, srchash, srci.Size(), "", false, false, srcsymlink))
    p.TotalBytes += uint64(srci.Size())
    return res

  }

  //
  // [Bidir] File in destination but not in source
  //

  if (p.Bidir || p.Dedup != nil) && os.IsNotExist(srcerr) && dsterr == nil {

    // Synchronize in the other direction
    if p.Bidir {
      var dsthash []byte
      if ! dsti.IsDir() {
        dsthash, err = repo.GetHash(dst, dsti, p.Dedup != nil)
        if err != nil {
          return p.HandleError(err)
        }
      }

      res := p.HandleAction(*NewCopyAction(dst, src, dsthash, dsti.Size(), "", false, false, dstsymlink))
      p.TotalBytes += uint64(dsti.Size())
      return res
    }

    // Record dst hash in case we move it
    if p.Dedup != nil {
      hash, err := repo.GetHash(dst, dsti, p.CheckHash)
      if err != nil {
        if ! p.HandleError(err) {
          return false
        }
      } else {
        p.Dedup[string(hash)] = append(p.Dedup[string(hash)], dst)
      }
    }
  }

  //
  // Handle stat() errors
  //

  if srcerr != nil {
    return p.HandleError(srcerr)
  }

  if dsterr != nil {
    return p.HandleError(dsterr)
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
      return p.HandleError(err)
    }
    defer f.Close()
    names, err := f.Readdirnames(-1)
    if err != nil {
      return p.HandleError(err)
    }

    for _, name := range names {
      if p.Bidir {
        srcnames[name] = true
      }
      if ! p.PrepareCopy(filepath.Join(src, name), filepath.Join(dst, name)) {
        return false
      }
    }

    if p.Bidir {

      f, err := os.Open(dst)
      if err != nil {
        return p.HandleError(err)
      }
      defer f.Close()
      dstnames, err := f.Readdirnames(-1)
      if err != nil {
        return p.HandleError(err)
      }

      for _, name := range dstnames {
        if srcnames[name] {
          continue
        }
        if ! p.PrepareCopy(filepath.Join(src, name), filepath.Join(dst, name)) {
          return false
        }
      }

    }

    return true

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
      srch, err = repo.HashFile(src, srci)
      computed = true
    }
    if err == nil && computed && p.Commit {
      _, err = repo.CommitFileHash(src, srci, srch, false)
    }
    if err != nil {
      return p.HandleError(err)
    }
  }
  if ! dsti.IsDir() {
    dsth, err = repo.GetHash(dst, dsti, false)
    computed := false
    if err == nil && dsth == nil {
      if p.Logger != nil {
        p.Logger(p, src, dst, false, true)
      }
      dsth, err = repo.HashFile(dst, dsti)
      computed = true
    }
    if err == nil && computed && p.Commit {
      _, err = repo.CommitFileHash(dst, dsti, dsth, false)
    }
    if err != nil {
      return p.HandleError(err)
    }
  }
  if bytes.Equal(srch, dsth) && srci.Mode() & os.ModeSymlink == dsti.Mode() & os.ModeSymlink {
    return true
  }

  if repo.ConflictFile(src) == "" {
    p.TotalBytes += uint64(srci.Size())
    dstname := repo.FindConflictFileName(dst, base58.Encode(srch))
    if ! p.HandleAction(*NewCopyAction(src, dstname, nil, srci.Size(), dst, true, false, srcsymlink)) {
      return false
    }
  }

  if p.Bidir && repo.ConflictFile(dst) == "" {
    p.TotalBytes += uint64(dsti.Size())
    srcname := repo.FindConflictFileName(src, base58.Encode(dsth))
    if ! p.HandleAction(*NewCopyAction(dst, srcname, nil, dsti.Size(), src, true, false, dstsymlink)) {
      return false
    }
  }

  return true
}
