package main

import (
  "flag"
  "fmt"
  "os"
  "io"
  "time"
  "path"
  "bytes"
  "path/filepath"
  "crypto/sha1"

  xattr "github.com/ivaxer/go-xattr"
  mh "github.com/jbenet/go-multihash"
  base58 "github.com/jbenet/go-base58"
)

const DirStoreName string = ".dirstore"
const XattrHash string = "user.doc.multihash"
const XattrHashTime string = "user.doc.multihash.time"

var commands map[string]func([]string)

func init(){
  commands = map[string]func([]string){
    "help": mainHelp,
    "init": mainInit,
    "status": mainStatus,
    "check": mainCheck,
    "commit": mainCommit,
  }
}

func main() {
  //f := flag.NewFlagSet("doc flags")
  flag.Parse()

  f := commands[flag.Arg(0)]
  if f == nil {
    mainHelp(nil)
  } else {
    f(flag.Args()[1:])
  }
}

func mainHelp([]string) {
  for cmd, _ := range commands {
    fmt.Printf("%s\n", cmd)
  }
}

func mainInit(args []string) {
  f := flag.NewFlagSet("init", flag.ExitOnError)
  f.Parse(args)
  dir := f.Arg(0)
  if dir == "" {
    dir = "."
  }

  dirstore := path.Join(dir, DirStoreName)
  os.Mkdir(dirstore, 0777)
}

func mainCheck(args []string) {
  f := flag.NewFlagSet("status", flag.ExitOnError)
  opt_all := f.Bool("a", false, "Check all files, including modified")
  f.Parse(args)
  dir := f.Arg(0)
  if dir == "" {
    dir = "."
  }

  err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
    if err != nil {
      return err
    }

    // Skip .dirstore/ at root
    if filepath.Base(path) == DirStoreName && filepath.Dir(path) == dir && info.IsDir() {
      return filepath.SkipDir
    } else if info.IsDir() {
      return nil
    }

    hashTimeStr, err := xattr.Get(path, XattrHashTime)
    if err != nil {
      return nil
    }

    hashTime, err := time.Parse(time.RFC3339Nano, string(hashTimeStr))
    if err != nil {
      return err
    }

    timeEqual := hashTime == info.ModTime()
    if *opt_all || timeEqual {

      hash, err := xattr.Get(path, XattrHash)
      if err != nil {
        return err
      }

      digest, err := hashFile(path)
      if err != nil {
        return err
      }

      hashEqual := bytes.Equal(hash, digest)

      if !timeEqual && !hashEqual {
        fmt.Printf("+\t%s\t%s\n", base58.Encode(digest), path)
      } else if !hashEqual {
        fmt.Printf("!\t%s\t%s\n", base58.Encode(digest), path)
      } else if !timeEqual {
        fmt.Printf("=\t%s\t%s", base58.Encode(digest), path)
      }
    }

    return nil
  })

  if err != nil {
    fmt.Errorf("%v", err)
    os.Exit(1)
  }
}

func mainStatus(args []string) {
  f := flag.NewFlagSet("status", flag.ExitOnError)
  f.Parse(args)
  dir := f.Arg(0)
  if dir == "" {
    dir = "."
  }

  err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
    if err != nil {
      return err
    }

    // Skip .dirstore/ at root
    if filepath.Base(path) == DirStoreName && filepath.Dir(path) == dir && info.IsDir() {
      return filepath.SkipDir
    } else if info.IsDir() {
      return nil
    }

    hashTimeStr, err := xattr.Get(path, XattrHashTime)
    if err != nil {
      fmt.Printf("?\t%s\n", path)
    } else {
      hashTime, err := time.Parse(time.RFC3339Nano, string(hashTimeStr))
      if err != nil {
        return err
      }

      if hashTime != info.ModTime() {
        fmt.Printf("+\t%s\n", path)
      }
    }

    return nil
  })

  if err != nil {
    fmt.Errorf("%v", err)
    os.Exit(1)
  }
}

func mainCommit(args []string) {
  f := flag.NewFlagSet("status", flag.ExitOnError)
  f.Parse(args)
  dir := f.Arg(0)
  if dir == "" {
    dir = "."
  }

  err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
    if err != nil {
      return err
    }

    // Skip .dirstore/ at root
    if filepath.Base(path) == DirStoreName && filepath.Dir(path) == dir && info.IsDir() {
      return filepath.SkipDir
    } else if info.IsDir() {
      return nil
    }

    digest, err := hashFile(path)
    if err != nil {
      return err
    }

    hash, err := xattr.Get(path, XattrHash)
    if err != nil || !bytes.Equal(hash, digest) {
      fmt.Printf("%s %s\n", base58.Encode(digest), path)
      xattr.Set(path, XattrHash, digest)
    }

    xattr.Set(path, XattrHashTime, []byte(info.ModTime().Format(time.RFC3339Nano)))
    return nil
  })

  if err != nil {
    fmt.Errorf("%v", err)
    os.Exit(1)
  }
}

func hashFile(path string) (mh.Multihash, error) {
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


