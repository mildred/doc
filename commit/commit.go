package commit

import (
  "io"
  "os"
  "fmt"
  "bufio"
  "bytes"
  "strings"
  "hash"
  "crypto/sha1"

  mh "github.com/jbenet/go-multihash"
  repo "github.com/mildred/doc/repo"
  attrs "github.com/mildred/doc/attrs"
  base58 "github.com/jbenet/go-base58"
)

const XattrCommit string = "user.doc.commit"

type CommitFileWriter struct {
  file io.WriteCloser
  path string
  hasher hash.Hash
}

func Read(path string) (map[string][]string, error) {
  f, err := os.Open(path)
  if err != nil {
    return nil, err
  }
  defer f.Close()

  res := map[string][]string{}

  scanner := bufio.NewScanner(f)
  for scanner.Scan() {
    line := scanner.Text()
    elems := strings.SplitN(line, "\t", 2)
    res[elems[0]] = append(res[elems[0]], DecodePath(elems[1]))
  }

  return res, scanner.Err()
}

func Create(path string) (*CommitFileWriter, error) {
  info, err := os.Lstat(path)
  if ! os.IsNotExist(err) {
    hash, err := attrs.Get(path, XattrCommit)
    if err != nil {
      return nil, fmt.Errorf("%s: manually modified (%s)", path, err.Error())
    }

    if len(hash) > 0 {
      hash2, err := repo.GetHash(path, info, false)
      if err != nil {
        return nil, fmt.Errorf("%s: cannot find hash (%s)", err.Error())
      }

      if ! bytes.Equal(hash, hash2) {
        return nil, fmt.Errorf("%s: file already exists and was manually modified (hash %s != %s)", path, base58.Encode(hash), base58.Encode(hash2))
      }
    }
  }
  f, err := os.Create(path)
  if err != nil {
    return nil, err
  }

  err = attrs.Set(path, XattrCommit, []byte{})
  if err != nil {
    f.Close()
    os.Remove(path)
    return nil, err
  }

  return &CommitFileWriter{ f, path, sha1.New() }, nil
}

func (c *CommitFileWriter) Close() error {
  err := c.file.Close()
  if err != nil {
    return err
  }

  info, err := os.Stat(c.path)
  if err != nil {
    return err
  }

  digest, err := mh.Encode(c.hasher.Sum(nil), mh.SHA1)
  if err != nil {
    panic(err)
  }

  err = attrs.Set(c.path, XattrCommit, digest)
  if err != nil {
    return err
  }

  _, err = repo.CommitFileHash(c.path, info, digest, false)
  if err != nil {
    return err
  }
  return nil
}

func (c *CommitFileWriter) AddEntry(hash []byte, path string) error {
  data := []byte(fmt.Sprintf("%s\t%s\n", base58.Encode(hash), EncodePath(path)))
  _, err := c.hasher.Write(data)
  if err != nil {
    panic (err)
  }

  _, err = c.file.Write(data)
  if err != nil {
    return err
  }
  return nil
}

func DecodePath(path string) string {
  path = strings.Replace(path, "\\\t", "\t", -1)
  path = strings.Replace(path, "\\\n", "\n", -1)
  path = strings.Replace(path, "\\\\", "\\", -1)
  return path
}

func EncodePath(path string) string {
  path = strings.Replace(path, "\\", "\\\\", -1)
  path = strings.Replace(path, "\t", "\\t", -1)
  path = strings.Replace(path, "\n", "\\n", -1)
  return path
}

