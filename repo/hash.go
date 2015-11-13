package repo

import (
  "os"
  "io"
  "time"
  "bytes"
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

func HashFile(path string, info os.FileInfo) (mh.Multihash, error) {
  if info.Mode() & os.ModeSymlink != 0 {
    return symlinkHash(path)
  }

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

func symlinkHash(path string) (mh.Multihash, error) {
  link, err := os.Readlink(path)
  if err != nil {
    return nil, err
  }

  hasher := sha1.New()
  _, err = hasher.Write([]byte(link))
  if err != nil {
    panic(err);
  }

  return mh.Encode(hasher.Sum(nil), mh.SHA1)
}

// Return the hash for path stored in the xattrs. If the hash is out of date,
// the hash is computed anew, unless `compute` is false in which case nil is
// returned.
func GetHash(path string, info os.FileInfo, compute bool) (mh.Multihash, error) {
  if info.Mode() & os.ModeSymlink != 0 {
    return symlinkHash(path)
  }

  hashTimeStr, err := attrs.Get(path, XattrHashTime)
  if err != nil {
    if compute {
      return HashFile(path, info)
    } else {
      // ignore error
      return nil, nil
    }
  }

  hashTime, err := time.Parse(time.RFC3339Nano, string(hashTimeStr))
  if err != nil {
    return nil, err
  }

  if hashTime != info.ModTime() {
    if compute {
      return HashFile(path, info)
    } else {
      return nil, nil
    }
  }

  return attrs.Get(path, XattrHash)
}

func CommitFileHash(path string, info os.FileInfo, digest []byte, force bool) (forced bool, err error) {
  timeData := []byte(info.ModTime().Format(time.RFC3339Nano))

  hash, err := attrs.Get(path, XattrHash)
  if err != nil || !bytes.Equal(hash, digest) {
    forced, err = attrs.SetForce(path, XattrHash, digest, info, force)
  } else {
    digest = nil
  }

  hashTimeStr, err := attrs.Get(path, XattrHashTime)
  var hashTime time.Time
  if err == nil {
    hashTime, err = time.Parse(time.RFC3339Nano, string(hashTimeStr))
  }
  if err != nil || hashTime != info.ModTime() {
    forced, err = attrs.SetForce(path, XattrHashTime, timeData, info, force)
  }

  return
}

