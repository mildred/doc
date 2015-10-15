package repo

import (
  "os"
  "io"
  "time"
  "crypto/sha1"

  mh "github.com/jbenet/go-multihash"
  xattr "github.com/ivaxer/go-xattr"
)

const XATTR_CREATE  = 1
const XATTR_REPLACE = 2

const DirStoreName string = ".dirstore"
const XattrHash string = "user.doc.multihash"
const XattrHashTime string = "user.doc.multihash.time"
const XattrConflict string = "user.doc.conflict"
const XattrAlternative string = "user.doc.alternative"

func HashFile(path string) (mh.Multihash, error) {
  f, err := os.Open(path)
  if err != nil {
    return nil, err
  }
  defer f.Close()

  hasher := sha1.New()
  _, err = io.Copy(hasher, f)
  if err != nil {
    return nil, err
  }

  digest, err := mh.Encode(hasher.Sum(nil), mh.SHA1)
  if err != nil {
    panic(err)
  }

  return digest, nil
}

func GetHash(path string, info os.FileInfo) (mh.Multihash, error) {
  hashTimeStr, err := xattr.Get(path, XattrHashTime)
  if err != nil {
    return HashFile(path)
  }

  hashTime, err := time.Parse(time.RFC3339Nano, string(hashTimeStr))
  if err != nil {
    return nil, err
  }

  if hashTime != info.ModTime() {
    return HashFile(path)
  }

  return xattr.Get(path, XattrHash)
}


