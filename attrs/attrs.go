package attrs

import (
	"fmt"
	xattr "github.com/ivaxer/go-xattr"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const DirStoreName string = ".dirstore"

const XATTR_CREATE = 1
const XATTR_REPLACE = 2

func IsErrno(err, errno error) bool {
	if e, ok := err.(*os.PathError); ok {
		return e.Err == errno
	} else {
		return err == errno
	}
}

// Return a list of extended attribute names for a path
func GetNameList(path string) ([]string, error) {
	list, err := xattr.List(path)
	if err == nil || !IsErrno(err, syscall.ENOTSUP) {
		return list, err
	} else {
		attrdir, _, err := findAttrDir(path)
		if err != nil {
			return nil, err
		} else if attrdir == "" {
			return nil, fmt.Errorf("%s: Could not find %s", path, DirStoreName)
		}

		f, err := os.Open(attrdir)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		names, err := f.Readdirnames(-1)
		if err != nil {
			return nil, err
		}

		var res []string

		for _, name := range names {
			if strings.HasSuffix(name, ".xattr") {
				res = append(res, name[:len(name)-6])
			}
		}

		return res, nil
	}
}

// Return a lit of attribute names and a list of their values
func GetList(path string) ([]string, [][]byte, error) {
	var val []byte
	list, err := GetNameList(path)
	values := make([][]byte, len(list))
	if err != nil {
		return nil, nil, err
	}
	for i, key := range list {
		val, err = xattr.Get(path, key)
		if err != nil {
			return nil, nil, err
		}
		values[i] = val
	}
	return list, values, nil
}

// Return an emty string if the dir store does not exists
func FindDirStore(path string) string {
	res := filepath.Join(path, DirStoreName)
	_, err := os.Lstat(path)
	if err != nil {
		return ""
	}

	_, err = os.Lstat(res)
	if err != nil {
		newpath, err := filepath.Abs(filepath.Join(path, ".."))
		if err != nil || newpath == path {
			return ""
		}
		return FindDirStore(newpath)
	}

	return res
}

func findAttrDir(path string) (string, os.FileInfo, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return "", nil, err
	}

	storepath := FindDirStore(path)
	if storepath == "" {
		return "", st, nil
	}

	storepath, err = filepath.Abs(storepath)
	if err != nil {
		return "", st, err
	}

	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		panic("Cannot read inode number")
	}
	inodenum := sys.Ino

	inodepath := filepath.Join(storepath, fmt.Sprintf("%d.inode", inodenum))

	err = os.MkdirAll(inodepath, os.ModePerm)
	if err != nil {
		return "", st, err
	}

	return inodepath, st, err
}

func findAttrFile(path, name string) (string, os.FileMode, error) {
	inodepath, st, err := findAttrDir(path)
	storepath := filepath.Dir(inodepath)

	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		panic("Cannot read inode number")
	}
	inodenum := sys.Ino

	xattrpath := filepath.Join(inodepath, fmt.Sprintf("%s.xattr", name))
	oldxattr := filepath.Join(storepath, fmt.Sprintf("inode.%d.%s.xattr", inodenum, name))

	if _, e := os.Stat(oldxattr); e == nil || !os.IsNotExist(e) {
		err = os.Rename(oldxattr, xattrpath)
	}

	return xattrpath, st.Mode(), err
}

func setAttr(path, name string, value []byte, flag int) error {
	attrname, mode, err := findAttrFile(path, name)
	if err != nil {
		return err
	} else if attrname == "" {
		return fmt.Errorf("%s: Could not find %s", path, DirStoreName)
	}

	f, err := os.OpenFile(attrname, flag|os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
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
		err = setAttr(path, name, value, os.O_CREATE|os.O_EXCL)
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
		e1 := os.Chmod(path, m|0200)
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
