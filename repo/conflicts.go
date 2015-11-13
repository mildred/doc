package repo

import (
  "os"
  "fmt"
  "bytes"
  "syscall"
  "path/filepath"

  attrs "github.com/mildred/doc/attrs"
  base58 "github.com/jbenet/go-base58"
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

// Return a conflict filename to use. Return the empty string if the conflict
// file already exists for the same hash.
func FindConflictFileName(path string, digest []byte) string {
  hashname := base58.Encode(digest)
  hashext := ""
  if len(hashname) != 0 {
    hashext = "." + hashname
  }
  ext := filepath.Ext(path)
  dstname := fmt.Sprintf("%s%s%s", path, hashext, ext)
  i := 0;
  for {
    info, err := os.Lstat(dstname)
    if os.IsNotExist(err) {
      return dstname
    }
    hash, err := GetHash(dstname, info, false)
    if err == nil && bytes.Equal(hash, digest) {
      return ""
    }
    dstname = fmt.Sprintf("%s%s.%d%s", path, hashext, i, ext)
    i++
  }
}

