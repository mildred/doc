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
  Dsthash []byte
  Size int64
  OriginalDst string
  Conflict bool
  Link bool
  NoXattr bool
}

func NewCopyAction(
  src string,
  dst string,
  dsthash []byte,
  size int64,
  originaldst string,
  conflict bool,
  link bool,
  noxattr bool) *CopyAction {
  return &CopyAction{src, dst, dsthash, size, originaldst, conflict, link, noxattr}
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
  if ! act.NoXattr {
    hash, err := attrs.Get(act.Src, repo.XattrHash)
    if err != nil {
      return err
    }
    hashTime, err := attrs.Get(act.Src, repo.XattrHashTime)
    if err != nil {
      return err
    }
    err = attrs.Set(act.Dst, repo.XattrHash, hash)
    if err != nil {
      return err
    }
    err = attrs.Set(act.Dst, repo.XattrHashTime, hashTime)
    if err != nil {
      return err
    }
    if act.Conflict {
      err = repo.MarkConflictFor(act.Dst, filepath.Base(act.OriginalDst))
      if err != nil {
        return err
      }
      err = repo.AddConflictAlternative(act.OriginalDst, filepath.Base(act.Dst))
      if err != nil {
        return err
      }
    }
  }
  return nil
}
