package sync

import (
  "os"
  "fmt"
  "os/exec"
  "path/filepath"

  "github.com/mildred/doc/repo"
  "github.com/mildred/doc/attrs"
)

type CopyAction struct {
  Src string
  Dst string
  Hash []byte
  Size int64
  OriginalDst string
  Conflict bool
  Link bool
  SrcMode os.FileMode
  OrigDstMode os.FileMode
}

func NewCopyAction(
  src string,
  dst string,
  hash []byte,
  size int64,
  originaldst string,
  conflict bool,
  link bool,
  srcMode os.FileMode,
  origDstMode os.FileMode) *CopyAction {
  return &CopyAction{src, dst, hash, size, originaldst, conflict, link, srcMode, origDstMode}
}

func (act *CopyAction) IsConflict() bool {
  return act.Conflict
}

func (act *CopyAction) Show() string {
  if act.Link {
    return fmt.Sprintf("ln %s %s\n", act.Src, act.Dst)
  } else {
    return fmt.Sprintf("cp -a --reflink=auto %s %s\n", act.Src, act.Dst)
  }
}

func (act *CopyAction) Run() error {
  var err error
  if act.Link {
    err = os.Link(act.Src, act.Dst)
    if err != nil {
      return fmt.Errorf("link %s: %s", act.Dst, err.Error())
    }
  } else {
    cmd := exec.Command("cp", "-a", "--reflink=auto", "-d", act.Src, act.Dst)
    cmd.Stderr = os.Stderr
    err = cmd.Run()
    if err != nil {
      return fmt.Errorf("cp %s: %s", act.Dst, err.Error())
    }
  }
  if act.Conflict {
    if act.SrcMode & os.ModeSymlink == 0 {
      err = repo.MarkConflictFor(act.Dst, filepath.Base(act.OriginalDst))
      if err != nil {
        return fmt.Errorf("%s: could not mark conflict: %s", act.Dst, err.Error())
      }
    }
    if act.OrigDstMode & os.ModeSymlink == 0 {
      err = repo.AddConflictAlternative(act.OriginalDst, filepath.Base(act.Dst))
      if err != nil {
        return fmt.Errorf("%s: could add conflict alternative: %s", act.Dst, err.Error())
      }
    }
  }
  if act.SrcMode & os.ModeSymlink == 0 {
    if act.Hash != nil {
      info, err := os.Lstat(act.Dst)
      if err != nil {
        return fmt.Errorf("%s: could add lstat: %s", act.Dst, err.Error())
      }
      _, err = repo.CommitFileHash(act.Dst, info, act.Hash, false)
      if err != nil {
        return fmt.Errorf("%s: could not commit: %s", act.Dst, err.Error())
      }
    } else {
      hash, err := attrs.Get(act.Src, repo.XattrHash)
      if err == nil {
        err = attrs.Set(act.Dst, repo.XattrHash, hash)
        if err != nil {
          return fmt.Errorf("%s: could add xattr %s: %s", act.Dst, repo.XattrHash, err.Error())
        }
      }
      hashTime, err := attrs.Get(act.Src, repo.XattrHashTime)
      if err == nil {
        err = attrs.Set(act.Dst, repo.XattrHashTime, hashTime)
        if err != nil {
          return fmt.Errorf("%s: could add xattr %s: %s", act.Dst, repo.XattrHashTime, err.Error())
        }
      }
    }
  }
  return nil
}
