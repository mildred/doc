package copy

import (
	"bytes"
	"path/filepath"

	"github.com/mildred/doc/commit"
)

func Copy(srcdir, dstdir string) (error, []error) {
	src, err := commit.ReadCommit(srcdir)
	if err != nil {
		return err, nil
	}
	dst, err := commit.ReadCommit(dstdir)
	if err != nil {
		return err, nil
	}
	return copyCommits(srcdir, dstdir, src, dst)
}

func copyCommits(srcdir, dstdir string, src, dst *commit.Commit) (error, []error) {
	successes, err, errs := copyTree(srcdir, dstdir, src, dst)
	if err == nil {
		return err, errs
	}

	err = commit.WriteDirAppend(dstdir, successes)
	return err, errs
}

func copyTree(srcdir, dstdir string, src, dst *commit.Commit) ([]commit.Entry, error, []error) {
	var errs []error
	var success []commit.Entry
	okdirs := map[string]bool{}

	c, err := commit.OpenDir(dstdir)
	if err != nil {
		return success, err, errs
	}
	defer c.Close()

	for _, s := range src.Entries {

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
		di, hasd := dst.ByPath[s.Path]
		if hasd && bytes.Equal(dst.Entries[di].Hash, s.Hash) {
			continue
		}

		// Find destination file name
		d := commit.Entry{
			s.Hash,
			"",
		}
		if hasd {
			d.Path = commit.FindConflictFileName(s, dst)
		}

		if d.Path == "" {
			d.Path = s.Path
		}

		// Copy file
		err, ers := CopyFileNoReplace(filepath.Join(srcdir, s.Path), filepath.Join(dstdir, d.Path))
		errs = append(errs, ers...)
		if err != nil {
			return success, err, errs
		}

		// FIXME: in case of conflicts, mark the file as a conflict

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