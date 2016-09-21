package copy

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mildred/doc/commit"
	"github.com/mildred/doc/repo"
)

type Progress interface {
	SetProgress(cur, max int, message string)
}

func Copy(srcdir, dstdir string, p Progress) (error, []error) {
	if p != nil {
		p.SetProgress(0, 4, "Read commit "+srcdir)
	}

	src, err := commit.ReadCommit(srcdir)
	if err != nil {
		return err, nil
	}

	if p != nil {
		p.SetProgress(1, 4, "Read commit "+dstdir)
	}

	os.MkdirAll(dstdir, 0777)

	dst, err := commit.ReadCommit(dstdir)
	if err != nil {
		return err, nil
	}

	if p != nil {
		p.SetProgress(2, 4, "Prepare copy")
	}

	successes, err, errs := copyTree(srcdir, dstdir, src, dst, p)
	if err != nil {
		return err, errs
	}

	if p != nil {
		p.SetProgress(len(successes)+3, len(successes)+4, fmt.Sprintf("Commit %d new files to %#v", len(successes), dstdir))
	}

	err = commit.WriteDirAppend(dstdir, successes)

	if p != nil && err == nil {
		p.SetProgress(len(successes)+4, len(successes)+4,
			fmt.Sprintf("%d files copied with %d errors", len(successes), len(errs)))
	}
	return err, errs
}

func canCopy(s commit.Entry, src, dst *commit.Commit) bool {
	// Already there, skip
	di, conflict := dst.ByPath[s.Path]
	if conflict && bytes.Equal(dst.Entries[di].Hash, s.Hash) {
		return false
	}

	// Source is private, skip
	is_private := src.GetAttr(s.Path, "private") == "1"
	if is_private {
		return false
	}

	// Destination is unwanted, skip
	is_wanted := dst.GetAttr(s.Path, "wanted") != "0"
	if !is_wanted {
		return false
	}

	return true
}

func copyTree(srcdir, dstdir string, src, dst *commit.Commit, p Progress) ([]commit.Entry, error, []error) {
	var errs []error
	var success []commit.Entry
	okdirs := map[string]bool{}

	if p != nil {
		p.SetProgress(2, 4, "Prepare copy: open "+dstdir)
	}

	c, err := commit.OpenDir(dstdir)
	if err != nil {
		return success, err, errs
	}
	defer c.Close()

	if p != nil {
		p.SetProgress(2, 4, "Prepare copy: compute how many files to copy")
	}

	numfiles := len(src.Entries)
	if p != nil {
		numfiles = 0
		for _, s := range src.Entries {
			if canCopy(s, src, dst) {
				numfiles = numfiles + 1
			}
		}
	}

	if p != nil {
		p.SetProgress(2, numfiles+4, fmt.Sprintf("Prepare copy: starting copy for %d files...", numfiles))
	}

	for _, s := range src.Entries {
		// Cannot copy, skip
		if !canCopy(s, src, dst) {
			continue
		}

		// Find destination file name
		var d commit.Entry = commit.Entry(s)
		_, conflict := dst.ByPath[s.Path]
		if conflict {
			d.Path = commit.FindConflictFileName(s, dst)
			if d.Path == "" {
				continue
			}
		}

		srcpath := filepath.Join(srcdir, s.Path)
		dstpath := filepath.Join(dstdir, d.Path)

		if p != nil {
			p.SetProgress(len(success)+3, numfiles+4, "Copy "+d.Path)
		}

		// Create parent dirs
		err, ers := makeParentDirs(srcdir, dstdir, s.Path, okdirs)
		errs = append(errs, ers...)
		if err != nil {
			return success, err, errs
		}

		// Copy file
		err, ers = CopyFileNoReplace(srcpath, dstpath)
		errs = append(errs, ers...)
		if err != nil {
			return success, err, errs
		}

		// In case of conflicts, mark the file as a conflict
		if conflict {
			errs = append(errs, repo.MarkConflict(filepath.Join(dstdir, s.Path), dstpath)...)
		}

		// Add to commit file
		err = c.Add(d)
		if err != nil {
			errs = append(errs, err)
		}

		success = append(success, d)
	}
	return success, nil, errs
}
