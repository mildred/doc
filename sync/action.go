package sync

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mildred/doc/attrs"
	"github.com/mildred/doc/repo"
)

type CopyAction struct {
	Src         string
	Dst         string
	Hash        []byte
	Size        int64
	OriginalDst string
	Conflict    bool
	Link        bool
	SrcMode     os.FileMode
	OrigDstMode os.FileMode
	manualMode  bool
	srcInfo     os.FileInfo
}

func NewCopyAction(
	src string,
	dst string,
	hash []byte,
	size int64,
	originaldst string,
	conflict bool,
	srcMode os.FileMode,
	origDstMode os.FileMode) *CopyAction {
	return &CopyAction{src, dst, hash, size, originaldst, conflict, false, srcMode, origDstMode, false, nil}
}

func NewCopyFile(
	src string,
	dst string,
	hash []byte,
	info os.FileInfo) *CopyAction {
	return &CopyAction{src, dst, hash, size(info), "", false, false, info.Mode(), 0, true, info}
}

func NewCreateDir(src string, dst string, srcInfo os.FileInfo) *CopyAction {
	return &CopyAction{
		src,
		dst,
		nil,
		0,
		"",
		false,
		false,
		srcInfo.Mode(),
		0,
		true,
		srcInfo,
	}
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
	} else if act.manualMode && act.srcInfo.Mode()&^(os.ModeDir /*|os.ModeSymlink*/) == 0 { // FIXME: enable symlinks
		stat, ok := act.srcInfo.Sys().(*syscall.Stat_t)

		if !ok {
			panic("Could not get Stat_t")
		}

		symlink := act.srcInfo.Mode()&os.ModeSymlink != 0

		if act.srcInfo.IsDir() {
			err = os.Mkdir(act.Dst, 0700)
			if err != nil {
				return err
			}
		} else if symlink {
			link, err := os.Readlink(act.Src)
			if err != nil {
				return err
			}

			err = os.Symlink(link, act.Dst)
			if err != nil {
				return err
			}
		} else {
			f, err := os.OpenFile(act.Dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
			if err != nil {
				return err
			}
			defer f.Close()

			f0, err := os.Open(act.Src)
			if err != nil {
				return err
			}
			defer f0.Close()

			_, err = io.Copy(f, f0)
			if err != nil {
				return err
			}
		}

		err = os.Lchown(act.Dst, int(stat.Uid), int(stat.Gid))
		if err != nil {
			log.Println(err)
			err = nil
		}

		if !symlink {

			atime := time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
			err = os.Chtimes(act.Dst, atime, act.srcInfo.ModTime())
			if err != nil {
				return err
			}

			err = os.Chmod(act.Dst, act.SrcMode)
			if err != nil {
				return err
			}

			// FIXME: extended attributes for symlinks
			// golang is missing some syscalls

			xattr, values, err := attrs.GetList(act.Src)
			if err != nil {
				return err
			}

			for i, attrname := range xattr {
				err = attrs.Set(act.Src, attrname, values[i])
				if err != nil {
					return err
				}
			}

		}

		return nil
	} else {
		cmd := exec.Command("cp", "-a", "--reflink=auto", "-d", act.Src, act.Dst)
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("cp %s: %s", act.Dst, err.Error())
		}
	}
	if act.Conflict {
		if act.SrcMode&os.ModeSymlink == 0 {
			err = repo.MarkConflictFor(act.Dst, filepath.Base(act.OriginalDst))
			if err != nil {
				return fmt.Errorf("%s: could not mark conflict: %s", act.Dst, err.Error())
			}
		}
		if act.OrigDstMode&os.ModeSymlink == 0 {
			err = repo.AddConflictAlternative(act.OriginalDst, filepath.Base(act.Dst))
			if err != nil {
				return fmt.Errorf("%s: could add conflict alternative: %s", act.Dst, err.Error())
			}
		}
	}
	if act.SrcMode&os.ModeSymlink == 0 {
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
