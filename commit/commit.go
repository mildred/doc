package commit

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	base58 "github.com/jbenet/go-base58"
	mh "github.com/jbenet/go-multihash"
	attrs "github.com/mildred/doc/attrs"
	repo "github.com/mildred/doc/repo"
)

const Doccommit string = ".doccommit"
const XattrCommit string = "user.doc.commit"

type Entry struct {
	Hash []byte
	Path string
}

func ReadDirByPath(dirPath string) (map[string]string, error) {
	return readByPath(filepath.Join(dirPath, Doccommit))
}

func ReadDirByHash(dirPath string) (map[string][]string, error) {
	return readByHash(filepath.Join(dirPath, Doccommit))
}

func WriteDir(dirPath string, entries []Entry) error {
	var data []byte

	for _, e := range entries {
		data = append(data, []byte(fmt.Sprintf("%s\t%s\n", base58.Encode(e.Hash), EncodePath(e.Path)))...)
	}

	digest, err := mh.Encode(sha1.New().Sum(data), mh.SHA1)
	if err != nil {
		panic(err)
	}

	f, err := ioutil.TempFile(dirPath, Doccommit)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		os.Remove(f.Name())
		return err
	}

	// Check that the .doccommit file is clean (not manually modified)

	newpath := filepath.Join(dirPath, Doccommit)
	info, err := os.Lstat(newpath)
	if !os.IsNotExist(err) {
		hash, err := attrs.Get(newpath, XattrCommit)
		if err != nil {
			return fmt.Errorf("%s: manually modified (%s)", newpath, err.Error())
		}

		if len(hash) > 0 {
			hash2, err := repo.GetHash(newpath, info, false)
			if err != nil {
				return fmt.Errorf("%s: cannot find hash (%s)", err.Error())
			}

			if !bytes.Equal(hash, hash2) {
				return fmt.Errorf("%s: file already exists and was manually modified (hash %s != %s)", newpath, base58.Encode(hash), base58.Encode(hash2))
			}
		}
	}

	// Rename the doccommit file in a single atomic operation

	err = os.Rename(f.Name(), newpath)
	if err != nil {
		os.Remove(f.Name())
		return err
	}

	// Set the XattrCommit

	err = attrs.Set(newpath, XattrCommit, digest)
	if err != nil {
		return err
	}

	// Commit the file to its extended attributes

	info, err = os.Stat(newpath)
	if err != nil {
		return err
	}

	_, err = repo.CommitFileHash(newpath, info, digest, false)
	if err != nil {
		return err
	}

	return nil
}

// hash is base58 encoded
func readByPath(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	res := map[string]string{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		elems := strings.SplitN(line, "\t", 2)
		res[DecodePath(elems[1])] = elems[0]
	}

	return res, scanner.Err()
}

// hash is base58 encoded
func readByHash(path string) (map[string][]string, error) {
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
