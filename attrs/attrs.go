package attrs

import (
	"io/ioutil"
	"os"
	"fmt"
	"syscall"
	"path/filepath"
  xattr "github.com/ivaxer/go-xattr"
)

const DirStoreName string = ".dirstore"

const XATTR_CREATE  = 1
const XATTR_REPLACE = 2

func IsErrno(err, errno error) bool {
	if e, ok := err.(*os.PathError); ok {
		return e.Err == errno
	} else {
		return err == errno
	}
}

func FindDirStore(path string) string {
	res := filepath.Join(path, DirStoreName)
	_, err := os.Lstat(path)
	if err != nil {
		return ""
	}

	_, err = os.Lstat(res)
	if err != nil {
		return FindDirStore(filepath.Join(path, ".."))
	}

	return res
}

func findAttrFile(path, name string) (string, os.FileMode, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return "", 0, err
	}

	storepath := FindDirStore(path)
	if storepath == "" {
		return "", st.Mode(), nil
	}

	storepath, err = filepath.Abs(storepath)
	if err != nil {
		return "", st.Mode(), err
	}

	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		panic("Cannot read inode number")
	}
	inodenum := sys.Ino
	return filepath.Join(storepath, fmt.Sprintf("inode.%d.%s.xattr", inodenum, name)), st.Mode(), nil
}

func setAttr(path, name string, value []byte, flag int) error {
	attrname, mode, err := findAttrFile(path, name)
	if err != nil {
		return err
	} else if attrname == "" {
		return fmt.Errorf("%s: Could not find %s", path, DirStoreName)
	}

	f, err := os.OpenFile(attrname, flag | os.O_CREATE | os.O_TRUNC | os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(value)
	if err != nil {
		return err
	}
	return nil
}

func getAttr(path, name string) ([]byte, error) {
	attrname, _, err := findAttrFile(path, name)
	if err != nil {
		return nil, err
	} else if attrname == "" {
		return nil, nil
	}

	f, err := os.Open(attrname)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ioutil.ReadAll(f)
}

func Set(path, name string, value []byte) error {
	err := xattr.Set(path, name, value)
	if IsErrno(err, syscall.ENOTSUP) {
		err = setAttr(path, name, value, 0)
	}
	return err
}

func Create(path, name string, value []byte) error {
	err := xattr.Setxattr(path, name, value, XATTR_CREATE)
	if IsErrno(err, syscall.ENOTSUP) {
		err = setAttr(path, name, value, os.O_CREATE | os.O_EXCL)
	}
	return err
}

func SetForce(path, name string, value []byte, info os.FileInfo, force bool) (bool, error) {
	err := xattr.Set(path, name, value)
	if IsErrno(err, syscall.ENOTSUP) {
		return false, setAttr(path, name, value, 0)
	}
	forced := false
	if err != nil && force && os.IsPermission(err) {
		m := info.Mode()
		forced = true
		e1 := os.Chmod(path, m | 0200)
		if e1 != nil {
			err = e1
		} else {
			err = xattr.Set(path, name, value)
			e2 := os.Chmod(path, m)
			if e2 != nil {
				panic(fmt.Errorf("%s: Could not chmod back to 0%o", path, m))
			}
		}
	}
	return forced, err
}

func Get(path, name string) ([]byte, error) {
	res, err := xattr.Get(path, name)
	if IsErrno(err, syscall.ENOTSUP) {
		res, err = getAttr(path, name)
		if err != nil {
			err = syscall.ENODATA
		}
	}
	return res, err
}

