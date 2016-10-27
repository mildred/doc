package commit

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	base58 "github.com/jbenet/go-base58"
	mh "github.com/jbenet/go-multihash"
	attrs "github.com/mildred/doc/attrs"
	docattr "github.com/mildred/doc/docattr"
	repo "github.com/mildred/doc/repo"
)

const Doccommit string = ".doccommit"
const XattrCommit string = "user.doc.commit"

type Entry struct {
	Hash []byte
	Path string
	Uuid string
}

func (e *Entry) HashText() string {
	return base58.Encode(e.Hash)
}

type Commit struct {
	Entries []Entry
	ByHash  map[string][]int
	ByPath  map[string]int
	ByUuid  map[string]int
	Attrs   map[string]map[string]string
}

func (c *Commit) GetAttr(file, name string) string {
	oldfile := ""
	for file != oldfile {
		attrs, ok := c.Attrs[file+"/"]
		if ok {
			attr, ok := attrs[name]
			if ok {
				return attr
			}
		}
		oldfile = file
		file = filepath.Dir(file)
	}
	attrs, ok := c.Attrs["/"]
	if ok {
		return attrs[name]
	}
	return ""
}

// Takes a canonical path
func findCommitFile(dir string) string {
	for {
		name := filepath.Join(dir, Doccommit)
		_, err := os.Lstat(name)
		if err == nil {
			return name
		}
		dir = filepath.Dir(dir)
		if dir == "/" || dir == "." {
			return ""
		}
	}
}

func makeCanonical(dir string) (string, error) {
	dir2, err := filepath.Abs(dir)
	if err != nil {
		return dir, err
	}
	dir3, err := filepath.EvalSymlinks(dir2)
	if err != nil {
		return dir2, err
	}
	return dir3, nil
}

func pathPrefix(basepath, subpath string) (string, error) {
	prefix, err := filepath.Rel(basepath, subpath)
	if err != nil {
		return "", err
	}
	if prefix == "." {
		prefix = ""
	} else if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}
	return prefix, nil
}

func ReadCommit(dirPath string) (*Commit, error) {
	dirPath, err := makeCanonical(dirPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	cfile := findCommitFile(dirPath)
	if cfile == "" {
		return &Commit{
			[]Entry{},
			map[string][]int{},
			map[string]int{},
			map[string]int{},
			map[string]map[string]string{},
		}, nil
	}

	prefix, err := pathPrefix(filepath.Dir(cfile), dirPath)
	if err != nil {
		return nil, err
	}

	c, files, err := readCommitFile(cfile, prefix)
	if err != nil {
		return nil, err
	}

	c.Attrs, err = docattr.ReadTree(filepath.Dir(cfile), prefix, files)
	return c, err
}

func readEntryAttr(ent *Entry, key, val string) {
	switch key {
	case "p":
		ent.Path = val
		break
	case "h":
		ent.Hash = base58.Decode(val)
		break
	case "i":
		ent.Uuid = val
		break
	default:
		break
	}
}

func readEntry(scanner *bufio.Scanner) (ent Entry, ent_hash string) {
	line := scanner.Text()
	if len(line) > 0 && line == "-" {
		// New style entries
		var key, val string
		for {
			if !scanner.Scan() {
				break
			}
			line = scanner.Text()
			if line == "" {
				break
			}
			if line[0] == '\t' {
				val += "\n" + line[1:]
			} else {
				if key != "" {
					readEntryAttr(&ent, key, val)
					key = ""
				}
				s := strings.SplitN(line, "=", 2)
				if len(s) == 2 {
					key = s[0]
					val = s[1]
				}
			}
		}
		if key != "" {
			readEntryAttr(&ent, key, val)
		}
	} else {
		elems := strings.SplitN(line, "\t", 2)
		f := DecodePath(elems[1])
		ent = Entry{
			Path: f,
			Hash: []byte(elems[0]),
		}
	}
	if ent.Path != "" {
		ent_hash = string(ent.Hash)
		ent.Hash = base58.Decode(ent_hash)
	}
	return
}

func readCommitFile(path, prefix string) (*Commit, []string, error) {
	var files []string
	c := Commit{
		nil,
		map[string][]int{},
		map[string]int{},
		map[string]int{},
		map[string]map[string]string{},
	}

	f, err := os.Open(path)
	if err != nil && os.IsNotExist(err) {
		return &c, nil, nil
	} else if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	idx := 0
	for scanner.Scan() {
		ent, ent_hash := readEntry(scanner)
		ent.Path = FilterPrefix(ent.Path, prefix, false)
		if ent.Path != "" {
			files = append(files, ent.Path)
			c.Entries = append(c.Entries, ent)
			c.ByPath[ent.Path] = idx
			c.ByHash[ent_hash] = append(c.ByHash[ent_hash], idx)
			if ent.Uuid != "" {
				c.ByUuid[ent.Uuid] = idx
			}
			idx = idx + 1
		}
	}

	return &c, files, scanner.Err()
}

type CommitAppender struct {
	f      *os.File
	prefix string
	first  bool
}

func OpenDir(dirPath string) (*CommitAppender, error) {
	dirPath, err := makeCanonical(dirPath)
	if err != nil {
		return nil, err
	}

	cfile := findCommitFile(dirPath)
	if cfile == "" {
		cfile = filepath.Join(dirPath, Doccommit)
	}

	prefix, err := pathPrefix(filepath.Dir(cfile), dirPath)
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile(cfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	return &CommitAppender{f, prefix, true}, nil
}

func (c *CommitAppender) Add(e Entry) error {
	if c.first {
		err := attrs.Set(c.f.Name(), XattrCommit, []byte{})
		if err != nil {
			return err
		}
		c.first = false
	}
	_, err := c.f.Write([]byte(entryToLine(e)))
	return err
}

func (c *CommitAppender) Close() error {
	return c.f.Close()
}

func WriteDir(dirPath string, entries []Entry) error {
	dirPath, err := makeCanonical(dirPath)
	if err != nil {
		return err
	}

	cfile := findCommitFile(dirPath)
	if cfile == "" {
		cfile = filepath.Join(dirPath, Doccommit)
	}

	prefix, err := pathPrefix(filepath.Dir(cfile), dirPath)
	if err != nil {
		return err
	}

	if prefix != "" {
		newEntries := entries

		// Read current entries
		entries, err = readEntries(cfile, prefix, true)
		if err != nil {
			return err
		}

		// Prefix the new entries
		for _, ent := range newEntries {
			ent.Path = prefix + ent.Path
			entries = append(entries, ent)
		}
	}

	return writeDoccommitFile(cfile, entries)
}

func WriteDirAppend(dirPath string, entries []Entry) error {
	dirPath, err := makeCanonical(dirPath)
	if err != nil {
		return err
	}

	cfile := findCommitFile(dirPath)
	if cfile == "" {
		cfile = filepath.Join(dirPath, Doccommit)
	}

	prefix, err := pathPrefix(filepath.Dir(cfile), dirPath)
	if err != nil {
		return err
	}

	newEntries := entries

	// Read current entries
	curEntries, err := readEntries(cfile, "", false)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Only include current entries that are not new
	entries = nil
	for _, ent := range curEntries {
		include := true
		for _, e := range newEntries {
			if e.Path == ent.Path {
				include = false
				break
			}
		}
		if include {
			entries = append(entries, ent)
		}
	}

	// Prefix the new entries
	for _, ent := range newEntries {
		ent.Path = prefix + ent.Path
		entries = append(entries, ent)
	}

	return writeDoccommitFile(cfile, entries)
}

func formatKeyVal(key, val string) string {
	if val == "" {
		return ""
	} else {
		return key + "=" + strings.Replace(val, "\n", "\n\t", -1) + "\n"
	}
}

func entryToLine(e Entry) string {
	if e.Uuid != "" {
		return "-\n" +
			formatKeyVal("p", e.Path) +
			formatKeyVal("h", base58.Encode(e.Hash)) +
			formatKeyVal("i", e.Uuid) +
			"\n"
	} else {
		return fmt.Sprintf("%s\t%s\n", base58.Encode(e.Hash), EncodePath(e.Path))
	}
}

func writeDoccommitFile(newpath string, entries []Entry) error {
	var data []byte

	for _, e := range entries {
		data = append(data, []byte(entryToLine(e))...)
	}

	digestBin := sha1.Sum(data)
	digest, err := mh.Encode(digestBin[:], mh.SHA1)
	if err != nil {
		return err
	}

	f, err := ioutil.TempFile(filepath.Dir(newpath), filepath.Base(newpath))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		os.Remove(f.Name())
		return err
	}

	// Rename the doccommit file in a single atomic operation
	err = os.Rename(f.Name(), newpath)
	if err != nil {
		os.Remove(f.Name())
		return err
	}

	return commitDircommit(newpath, digest)
}

func commitDircommit(newpath string, digest []byte) error {
	// Set the XattrCommit

	err := attrs.Set(newpath, XattrCommit, digest)
	if err != nil {
		return err
	}

	// Commit the file to its extended attributes

	info, err := os.Stat(newpath)
	if err != nil {
		return err
	}

	_, err = repo.CommitFileHash(newpath, info, digest, false)
	if err != nil {
		return err
	}

	return nil
}

func Init(dir string) error {
	newpath := filepath.Join(dir, Doccommit)

	// Rename the doccommit file in a single atomic operation

	f, err := os.OpenFile(newpath, os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	f.Close()

	digestBin := sha1.Sum([]byte{})
	digest, err := mh.Encode(digestBin[:], mh.SHA1)
	if err != nil {
		panic(err)
	}

	return commitDircommit(newpath, digest)
}

func readEntries(path, prefix string, reverse bool) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var res []Entry

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		ent, _ := readEntry(scanner)
		ent.Path = FilterPrefix(ent.Path, prefix, reverse)
		if ent.Path != "" {
			res = append(res, ent)
		}
	}

	return res, scanner.Err()
}

func FilterPrefix(path, prefix string, reverse bool) string {
	hasPrefix := prefix == "" || strings.HasPrefix(path, prefix)
	if !reverse {
		res, err := filepath.Rel(prefix, path)
		if err != nil {
			panic(err)
		}
		return res
	} else {
		if !hasPrefix {
			return path
		} else {
			return ""
		}
	}
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
