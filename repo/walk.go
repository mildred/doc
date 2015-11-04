package repo

import (
  "path/filepath"
  "strings"
  "os"

  attrs "github.com/mildred/doc/attrs"
)

type ErrorHandler func(path string, info os.FileInfo, err error)(cont bool)
type WalkHandler  func(path string, info os.FileInfo) error

type ErrorList []error

var SkipDir error = filepath.SkipDir

func (el ErrorList) Error() string {
  var l []string
  for _, e := range el {
    l = append(l, e.Error())
  }
  return strings.Join(l, "\n")
}

func Walk(dir string, wh WalkHandler, eh ErrorHandler) ErrorList {
  var el ErrorList = nil
  err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
    // Handle first error
    if err != nil {
      el = append(el, err)
      if ! eh(path, info, err) {
        return filepath.SkipDir
      } else {
        return nil
      }
    }

    // Skip .dirstore/ at root and non regular files
    if filepath.Base(path) == attrs.DirStoreName && filepath.Dir(path) == dir && info.IsDir() {
      return filepath.SkipDir
    } else if ! info.Mode().IsRegular() {
      return nil
    }

    // Handle file
    err = wh(path, info)
    if err == filepath.SkipDir {
      return err
    } else if err != nil {
      el = append(el, err)
      if ! eh(path, info, err) {
        return filepath.SkipDir
      } else {
        return nil
      }
    }

    return nil
  })
  if err != nil {
    panic(err) // Should not happen, out handler never return an error
  }
  return el
}
