package repo

import (
  "os"
  "fmt"
  "syscall"
  "path/filepath"

  attrs "github.com/mildred/doc/attrs"
)

func ConflictFile(path string) string {
  conflict, err := attrs.Get(path, XattrConflict)
  if err != nil {
    return ""
  } else {
    return string(conflict)
  }
}

func ConflictFileAlternatives(path string) []string {
  var alternatives []string
  for i := 0; true; i++ {
    alt, err := attrs.Get(path, fmt.Sprintf("%s.%d", XattrConflict, i))
    if err == nil {
      alternatives = append(alternatives, string(alt))
    } else {
      break
    }
  }
  return alternatives
}

func MarkConflictFor(path, conflictName string) error {
  return attrs.Set(path, XattrConflict, []byte(conflictName))
}

func AddConflictAlternative(path, alternativeName string) error {
  for i := 0; true; i++ {
    err := attrs.Create(path, fmt.Sprintf("%s.%d", XattrConflict, i), []byte(alternativeName))
    if err == nil {
      return nil
    } else if os.IsExist(err) || err == syscall.EEXIST {
      continue
    } else {
      return err
    }
  }
  return nil
}

func FindConflictFileName(path, hashname string) string {
  hashext := ""
  if len(hashname) != 0 {
    hashext = "." + hashname
  }
  ext := filepath.Ext(path)
  dstname := fmt.Sprintf("%s%s%s", path, hashext, ext)
  for i := 0; true; i++ {
    if _, err := os.Lstat(dstname); os.IsNotExist(err) {
      return dstname
    }
    dstname = fmt.Sprintf("%s%s.%d%s", path, hashext, i, ext)
  }
  return dstname
}

