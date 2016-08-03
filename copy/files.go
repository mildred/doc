package copy

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mildred/doc/attrs"
)

var ErrorExists = errors.New("File already exists")

func CopyFileNoReplace(src, dst string) (error, []error) {
	fname, err, errs := CopyFileTemp(src, dst)
	if err != nil {
		return err, errs
	}

	if _, err := os.Lstat(dst); err == nil || !os.IsNotExist(err) {
		if e := os.Remove(fname); e != nil {
			errs = append(errs, e)
		}
		if err == nil {
			err = ErrorExists
		}
		return err, errs
	}

	err = os.Rename(fname, dst)
	if err != nil {
		if e := os.Remove(fname); e != nil {
			errs = append(errs, e)
		}
	}

	return err, errs
}

func CopyFile(src, dst string) (error, []error) {
	fname, err, errs := CopyFileTemp(src, dst)
	if err != nil {
		return err, errs
	}

	err = os.Rename(fname, dst)
	if err != nil {
		if e := os.Remove(fname); e != nil {
			errs = append(errs, e)
		}
	}

	return err, errs
}

func CopyFileTemp(src, dst string) (string, error, []error) {
	var errs []error

	src_st, err := os.Lstat(src)
	if err != nil {
		return "", err, nil
	}
	symlink := src_st.Mode()&os.ModeSymlink != 0

	src_f, err := os.Open(src)
	if err != nil {
		return "", err, nil
	}

	f, err := ioutil.TempFile(filepath.Dir(dst), "temp")
	if err != nil {
		return "", err, nil
	}
	fname := f.Name()

	if symlink {
		f.Close()
		err = os.Remove(fname)
		if err != nil {
			return "", err, nil
		}

		target, err := os.Readlink(src)
		if err != nil {
			return "", err, nil
		}

		err = os.Symlink(target, fname)
		if err != nil {
			return "", err, nil
		}
	} else {
		defer f.Close()
		_, err = io.Copy(f, src_f)
		if err != nil {
			if e := os.Remove(fname); e != nil {
				errs = append(errs, e)
			}
			return "", err, errs
		}
	}

	if stat, ok := src_st.Sys().(*syscall.Stat_t); ok {

		err = os.Lchown(fname, int(stat.Uid), int(stat.Gid))
		if err != nil {
			errs = append(errs, err)
		}

		if !symlink {

			atime := time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
			err = os.Chtimes(fname, atime, src_st.ModTime())
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	if !symlink {

		err = os.Chmod(fname, src_st.Mode())
		if err != nil {
			errs = append(errs, err)
		}

		// FIXME: extended attributes for symlinks
		// golang is missing some syscalls

		xattr, values, err := attrs.GetList(src)
		if err != nil {
			errs = append(errs, err)
		} else {
			for i, attrname := range xattr {
				err = attrs.Set(fname, attrname, values[i])
				if err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	return fname, err, errs
}
