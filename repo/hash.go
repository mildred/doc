package repo

import (
  "os"
  "io"
  "time"
  "syscall"
  "crypto/sha1"

  mh "github.com/jbenet/go-multihash"
  attrs "github.com/mildred/doc/attrs"
)

const XattrHash string = "user.doc.multihash"
const XattrHashTime string = "user.doc.multihash.time"
const XattrConflict string = "user.doc.conflict"
const XattrAlternative string = "user.doc.alternative"

func IsNoData(err error) bool {
  return attrs.IsErrno(err, syscall.ENODATA)
}

func GetHashTime(path string) (time.Time, error) {
  hashTimeStr, err := attrs.Get(path, XattrHashTime)
  if err != nil {
    return time.Time{}, err
  }
  return time.Parse(time.RFC3339Nano, string(hashTimeStr))
}

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

func GetHash(path string, info os.FileInfo, compute bool) (mh.Multihash, error) {
  hashTimeStr, err := attrs.Get(path, XattrHashTime)
  if err != nil {
    return HashFile(path)
  }

  hashTime, err := time.Parse(time.RFC3339Nano, string(hashTimeStr))
  if err != nil {
    return nil, err
  }

  if hashTime != info.ModTime() {
    if compute {
      return HashFile(path)
    } else {
      return nil, nil
    }
  }

  return attrs.Get(path, XattrHash)
}


