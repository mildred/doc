package main

import (
  "flag"
  "fmt"
  "os"
  "io"
  "time"
  "path"
  "bytes"
  "os/exec"
  "crypto/sha1"
  "path/filepath"

  xattr "github.com/ivaxer/go-xattr"
  mh "github.com/jbenet/go-multihash"
  base58 "github.com/jbenet/go-base58"
)

const DirStoreName string = ".dirstore"
const XattrHash string = "user.doc.multihash"
const XattrHashTime string = "user.doc.multihash.time"
const XattrConflict string = "user.doc.conflict"
const XattrAlternative string = "user.doc.alternative"

var commands map[string]func([]string)

func init(){
  commands = map[string]func([]string){
    "help": mainHelp,
    "init": mainInit,
    "status": mainStatus,
    "check": mainCheck,
    "commit": mainCommit,
    "cp": mainCopy,
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
    fmt.Fprintf(os.Stderr, "%v", err)
    os.Exit(1)
  }
}

func conflictFile(path string) string {
  conflict, err := xattr.Get(path, XattrConflict)
  if err != nil {
    return ""
  } else {
    return string(conflict)
  }
}

func conflictFileAlternatives(path string) []string {
  var alternatives []string
  for i := 0; true; i++ {
    alt, err := xattr.Get(path, fmt.Sprintf("%s.%d", XattrConflict, i))
    if err != nil {
      alternatives = append(alternatives, string(alt))
    } else {
      break
    }
  }
  return alternatives
}

func mainStatus(args []string) {
  f := flag.NewFlagSet("status", flag.ExitOnError)
  f.Parse(args)
  dir := f.Arg(0)
  if dir == "" {
    dir = "."
  }

  status := 0

  err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
    if err != nil {
      fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
      status = 1
      return err
    }

    // Skip .dirstore/ at root
    if filepath.Base(path) == DirStoreName && filepath.Dir(path) == dir && info.IsDir() {
      return filepath.SkipDir
    } else if info.IsDir() {
      return nil
    }

    var conflict string = ""
    if conflictFile(path) != "" {
      conflict = " c"
    } else if len(conflictFileAlternatives(path)) > 0 {
      conflict = " C"
    }

    hashTimeStr, err := xattr.Get(path, XattrHashTime)
    if err != nil {
      if info.Mode() & os.FileMode(0200) == 0 {
        fmt.Printf("?%s (ro)\t%s\n", conflict, path)
      } else {
        fmt.Printf("?%s\t%s\n", conflict, path)
      }
    } else {
      hashTime, err := time.Parse(time.RFC3339Nano, string(hashTimeStr))
      if err != nil {
        fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
        return nil
      }

      if hashTime != info.ModTime() {
        fmt.Printf("+%s\t%s\n", conflict, path)
      } else if conflict != "" {
        fmt.Printf("%s\t%s\n", conflict, path)
      }
    }

    return nil
  })

  if err != nil {
    fmt.Fprintf(os.Stderr, "%v", err)
    os.Exit(1)
  }
  os.Exit(status)
}

func mainCommit(args []string) {
  f := flag.NewFlagSet("status", flag.ExitOnError)
  opt_force := f.Bool("f", false, "Force writing xattrs on read only files")
  f.Parse(args)
  dir := f.Arg(0)
  if dir == "" {
    dir = "."
  }

  status := 0

  err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
    if err != nil {
      fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
      status = 1
      return err
    }

    // Skip .dirstore/ at root
    if filepath.Base(path) == DirStoreName && filepath.Dir(path) == dir && info.IsDir() {
      return filepath.SkipDir
    } else if info.IsDir() {
      return nil
    }

    digest, err := commitFile(path, info, *opt_force)
    if err != nil {
      status = 1
      fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
    } else if digest != nil {
      fmt.Printf("%s %s\n", base58.Encode(digest), path)
    }

    return nil
  })

  if err != nil {
    fmt.Fprintf(os.Stderr, "%v", err)
    os.Exit(1)
  }

  os.Exit(status)
}

func mainCopy(args []string) {
  f := flag.NewFlagSet("cp", flag.ExitOnError)
  opt_dry_run := f.Bool("n", false, "Dry run")
  f.Parse(args)
  src := f.Arg(0)
  dst := f.Arg(1)

  if src == "" && dst == "" {
    fmt.Fprintln(os.Stderr, "You must specify at least the destination directory")
    os.Exit(1)
  } else if dst == "" {
    dst = src
    src = "."
  }

  status := 0

  /*var dirs []string

  err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
    if err != nil {
      fmt.Fprintf(os.Stderr, "%s: %v\n", path, err.Error())
      status = 1
      return err
    }

    // Skip .dirstore/ at root
    if filepath.Base(path) == DirStoreName && filepath.Dir(path) == src && info.IsDir() {
      return filepath.SkipDir
    }

    destpath := filepath.Join(append([]string{dst}, dirs...)...)

    / *
    if info.IsDir() {
      err = copyDir(path, destpath)
    } else {
      err = copyFile(path, destpath)
    }
    * /

    fmt.Printf("cp %s %s\n", path, destpath)

    if info.IsDir() {
      dirs = append(dirs, filepath.Base(path))
    }

    return nil
  })
  */

  conflicts, err := copyEntry(src, dst, *opt_dry_run)

  for _, c := range conflicts {
    fmt.Fprintf(os.Stderr, "CONFLICT %s\n", c)
  }

  if err != nil {
    fmt.Fprintf(os.Stderr, "%v", err)
    os.Exit(1)
  }

  os.Exit(status)
}

func copyEntry(src, dst string, dry_run bool) ([]string, error) {
  srci, err := os.Stat(src)
  if err != nil {
    return nil, err
  }

  dsti, err := os.Stat(dst)
  if os.IsNotExist(err) {
    if dry_run {
      fmt.Printf("cp -la %s %s\n", src, dst)
    } else {
      err = exec.Command("cp", "-la", src, dst).Run()
      if err != nil {
        return nil, err
      }
    }
    return nil, nil
  } else if srci.IsDir() && dsti.IsDir() {
    conflicts := []string{}
    f, err := os.Open(src)
    if err != nil {
      return nil, err
    }
    defer f.Close()
    names, err := f.Readdirnames(-1)
    if err != nil {
      return nil, err
    }
    for _, name := range names {
      c, err := copyEntry(filepath.Join(src, name), filepath.Join(dst, name), dry_run)
      if err != nil {
        return nil, err
      } else {
        conflicts = append(conflicts, c...)
      }
    }
    return conflicts, nil
  } else {
    var srch, dsth []byte
    if ! srci.IsDir() {
      srch, err = getHash(src, srci)
      if err != nil {
        return nil, err
      }
    }
    if ! dsti.IsDir() {
      dsth, err = getHash(dst, dsti)
      if err != nil {
        return nil, err
      }
    }
    if bytes.Equal(srch, dsth) {
      return nil, nil
    }

    dstname := findConflictFileName(dst, base58.Encode(srch))
    if dry_run {
      fmt.Printf("cp -la %s %s\n", src, dstname)
    } else {
      err = exec.Command("cp", "-la", src, dstname).Run()
      if err != nil {
        return nil, err
      }
    }
    return []string{dstname}, nil
  }
}

func findConflictFileName(path, hashname string) string {
  hashext := ""
  if len(hashname) != 0 {
    hashext = "." + hashname
  }
  ext := filepath.Ext(path)
  dstname := fmt.Sprintf("%s%s%s", path, hashext, ext)
  for i := 0; true; i++ {
    if _, err := os.Stat(dstname); os.IsNotExist(err) {
      return dstname
    }
    dstname = fmt.Sprintf("%s%s.%d%s", path, hashext, i, ext)
  }
  return dstname
}

func commitFile(path string, info os.FileInfo, force bool) ([]byte, error) {
  digest, err := hashFile(path)
  if err != nil {
    return nil, err
  }

  timeData := []byte(info.ModTime().Format(time.RFC3339Nano))

  hash, err := xattr.Get(path, XattrHash)
  if err != nil || !bytes.Equal(hash, digest) {
    err = xattr.Set(path, XattrHash, digest)
    if err != nil && force && os.IsPermission(err) {
      fmt.Fprintf(os.Stderr, "%s: force write xattrs\n", path)
      m := info.Mode()
      e1 := os.Chmod(path, m | 0200)
      if e1 != nil {
        err = e1
      } else {
        digest, err = commitFile(path, info, false)
        e2 := os.Chmod(path, m)
        if e2 != nil {
          err = e2
        }
      }
    } else if err != nil {
      return nil, err
    }
  } else {
    digest = nil
  }

  hashTimeStr, err := xattr.Get(path, XattrHashTime)
  var hashTime time.Time
  if err == nil {
    hashTime, err = time.Parse(time.RFC3339Nano, string(hashTimeStr))
  }
  if err != nil || hashTime != info.ModTime() {
    err = xattr.Set(path, XattrHashTime, timeData)
    if err != nil && force && os.IsPermission(err) {
      fmt.Fprintf(os.Stderr, "%s: force write xattrs\n", path)
      m := info.Mode()
      e1 := os.Chmod(path, m | 0200)
      if e1 != nil {
        err = e1
      } else {
        digest, err = commitFile(path, info, false)
        e2 := os.Chmod(path, m)
        if e2 != nil {
          err = e2
        }
      }
    }
  }

  return digest, err
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

func getHash(path string, info os.FileInfo) (mh.Multihash, error) {
  hashTimeStr, err := xattr.Get(path, XattrHashTime)
  if err != nil {
    return hashFile(path)
  }

  hashTime, err := time.Parse(time.RFC3339Nano, string(hashTimeStr))
  if err != nil {
    return nil, err
  }

  if hashTime != info.ModTime() {
    return hashFile(path)
  }

  return xattr.Get(path, XattrHash)
}

