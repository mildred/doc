package repo

import (
	"os"
	"os/exec"
	"path/filepath"

	base58 "github.com/jbenet/go-base58"
	attrs "github.com/mildred/doc/attrs"
)

type Par2Repo struct {
	repoPath string
}

func GetRepo(path string) *Par2Repo {
	repo := attrs.FindDirStore(path)
	if repo == "" {
		return nil
	} else {
		return &Par2Repo{repo}
	}
}

func (r *Par2Repo) HashFile(digest []byte) string {
	return filepath.Join(r.repoPath, base58.Encode(digest))
}

func (r *Par2Repo) Par2File(digest []byte) string {
	return r.HashFile(digest) + ".par2"
}

func (r *Par2Repo) Par2Exists(digest []byte) (bool, error) {
	_, err := os.Stat(r.Par2File(digest))
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func (r *Par2Repo) Create(path string, digest []byte) error {
	hashFile := r.HashFile(digest)
	par2file := hashFile + ".par2"
	err := os.Link(path, hashFile)
	if err != nil {
		return err
	}
	defer os.Remove(hashFile)
	cmd := exec.Command("par2create", "--", par2file, hashFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
