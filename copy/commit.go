package copy

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/mildred/doc/commit"
	"github.com/mildred/doc/repo"
)

type Progress interface {
	SetProgress(cur, max int, message string)
}

func Copy(srcdir, dstdir string, p Progress) (error, []error) {
	if p != nil {
		p.SetProgress(0, 3, "Read commit "+srcdir)
	}

	src, err := commit.ReadCommit(srcdir)
	if err != nil {
		return err, nil
	}

	if p != nil {
		p.SetProgress(1, 3, "Read commit "+dstdir)
	}

	dst, err := commit.ReadCommit(dstdir)
	if err != nil {
		return err, nil
	}

	successes, err, errs := copyTree(srcdir, dstdir, src, dst, p)
	if err != nil {
		return err, errs
	}

	if p != nil {
		p.SetProgress(len(successes)+2, len(successes)+3, "Commit destination")
	}

	err = commit.WriteDirAppend(dstdir, successes)

	if p != nil && err == nil {
		p.SetProgress(len(successes)+3, len(successes)+3,
			fmt.Sprintf("%d files copied with %d errors", len(successes), len(errs)))
	}
	return err, errs
}

func copyTree(srcdir, dstdir string, src, dst *commit.Commit, p Progress) ([]commit.Entry, error, []error) {
	var errs []error
	var success []commit.Entry
	okdirs := map[string]bool{}

	c, err := commit.OpenDir(dstdir)
	if err != nil {
		return success, err, errs
	}
	defer c.Close()

	numfiles := len(src.Entries)
	if p != nil {
		numfiles = 0
		for _, s := range src.Entries {
			di, conflict := dst.ByPath[s.Path]
			if !conflict || (!bytes.Equal(dst.Entries[di].Hash, s.Hash) &&
				commit.FindConflictFileName(s, dst) != "") {
				numfiles = numfiles + 1
			}
		}
	}

	for _, s := range src.Entries {
		// Already there, skip
		di, conflict := dst.ByPath[s.Path]
		if conflict && bytes.Equal(dst.Entries[di].Hash, s.Hash) {
			continue
		}

		// Find destination file name
		d := commit.Entry{
			s.Hash,
			s.Path,
		}
		if conflict {
			d.Path = commit.FindConflictFileName(s, dst)
			if d.Path == "" {
				continue
			}
		}

		srcpath := filepath.Join(srcdir, s.Path)
		dstpath := filepath.Join(dstdir, d.Path)

		if p != nil {
			p.SetProgress(len(success)+2, numfiles+3, "Copy "+d.Path)
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
