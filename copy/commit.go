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
	src, err := commit.ReadCommit(srcdir)
	if err != nil {
		return err, nil
	}
	dst, err := commit.ReadCommit(dstdir)
	if err != nil {
		return err, nil
	}
	return copyCommits(srcdir, dstdir, src, dst, p)
}

func copyCommits(srcdir, dstdir string, src, dst *commit.Commit, p Progress) (error, []error) {
	successes, err, errs := copyTree(srcdir, dstdir, src, dst, p)
	if err == nil {
		return err, errs
	}

	if p != nil {
		p.SetProgress(len(src.Entries), len(src.Entries)+1, "Commit destination")
	}

	err = commit.WriteDirAppend(dstdir, successes)

	if p != nil {
		p.SetProgress(len(src.Entries)+1, len(src.Entries)+1, "")
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

	for idx, s := range src.Entries {
		if p != nil {
			p.SetProgress(idx, len(src.Entries)+1, s.Path)
		}

		// Create parent dirs
		for _, dir := range parentDirs(s.Path, okdirs) {
			err, ers := MkdirFrom(filepath.Join(srcdir, dir), filepath.Join(dstdir, dir))
			errs = append(errs, ers...)
			if err != nil {
				return success, err, errs
			}
			okdirs[dir] = true
		}

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

		// Copy file
		err, ers := CopyFileNoReplace(srcpath, dstpath)
		errs = append(errs, ers...)
		if err != nil {
			return success, err, errs
		}

		// In case of conflicts, mark the file as a conflict
		if conflict {
			// FIXME: mark conflicts for symlinks as well when the syscall is
			// available
			parentfile := filepath.Join(dstdir, s.Path)

			dstpath_st, err := os.Lstat(dstpath)
			if err == nil && dstpath_st.Mode()&os.ModeSymlink == 0 {
				err = repo.MarkConflictFor(dstpath, filepath.Base(parentfile))
				if err != nil {
					errs = append(errs, fmt.Errorf("%s: could not mark conflict: %s", dstpath, err.Error()))
				}
			} else {
				errs = append(errs, err)
			}

			parentfile_st, err := os.Lstat(parentfile)
			if err == nil && parentfile_st.Mode()&os.ModeSymlink == 0 {
				err = repo.AddConflictAlternative(parentfile, filepath.Base(dstpath))
				if err != nil {
					errs = append(errs, fmt.Errorf("%s: could add conflict alternative: %s", dstpath, err.Error()))
				}
			} else {
				errs = append(errs, err)
			}
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

func copyEntries(src, dst string, entries []commit.Entry) ([]commit.Entry, error, []error) {
	var errs []error
	var success []commit.Entry
	okdirs := map[string]bool{}

	for _, e := range entries {
		// Create parent dirs
		for _, dir := range parentDirs(e.Path, okdirs) {
			err, ers := MkdirFrom(filepath.Join(src, dir), filepath.Join(dst, dir))
			errs = append(errs, ers...)
			if err != nil {
				return success, err, errs
			}
			okdirs[dir] = true
		}

		// Copy file
		err, ers := CopyFileNoReplace(filepath.Join(src, e.Path), filepath.Join(dst, e.Path))
		errs = append(errs, ers...)
		if err != nil {
			return success, err, errs
		}

		success = append(success, e)
	}

	return success, nil, errs
}

func parentDirs(path string, ok map[string]bool) []string {
	var res []string
	var breadcrumb []string

	d := filepath.Dir(path)
	for !ok[d] && d != "." && d != "/" {
		breadcrumb = append(breadcrumb, d)
		d = filepath.Dir(d)
	}

	// reverse
	for i := len(breadcrumb) - 1; i >= 0; i-- {
		res = append(res, breadcrumb[i])
	}
	return res
}
