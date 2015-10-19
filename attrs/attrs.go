package attrs

import (
	"os"
  xattr "github.com/ivaxer/go-xattr"
)

const XATTR_CREATE  = 1
const XATTR_REPLACE = 2

func Set(path, name string, value []byte) error {
	return xattr.Set(path, name, value)
}

func Create(path, name string, value []byte) error {
	return xattr.Setxattr(path, name, value, XATTR_CREATE)
}

func SetForce(path, name string, value []byte, info os.FileInfo, force bool) (bool, error) {
  err := xattr.Set(path, name, value)
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
        err = e2
      }
    }
  }
  return forced, err
}

func Get(path, name string) ([]byte, error) {
	return xattr.Get(path, name)
}

