package copy

import (
	"fmt"

	"github.com/mildred/doc/commit"
)

func copyTreeRename(srcdir, dstdir string, src, dst *commit.Commit, p Progress) ([]commit.Entry, error, []error) {
	var errs []error
	var success []commit.Entry

	if p != nil {
		p.SetProgress(2, 4, "Prepare copy: open "+dstdir)
	}

	c, err := commit.OpenDirAppend(dstdir)
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
			if wantCopy(s, src, dst) {
				numfiles = numfiles + 1
			}
		}
	}

	if p != nil {
		p.SetProgress(2, numfiles+4, fmt.Sprintf("Prepare copy: starting copy for %d files...", numfiles))
	}

	// loop over all sources entries, including directories (coming before files)

	for _, s := range src.Entries {

		// Don'r want to copy, skip
		if !wantCopy(s, src, dst) {
			continue
		}

		// DEFAULT DESTINATION:
		// the entry on the destination located on the same path than on source

		ddest_num, ddest_exists := dst.ByPath[s.Path]
		ddest := dst.Entries[ddest_num]

		// MATCHING DESTINATION:
		// if the source has an id, lookup an entry of the same type and id in the
		// destination. The search must be done in the destination parent directory as
		// far as possible (do not filter the .dircommit file on a subdirectory).
		// This is to prevent to have two different inodes with the same id.

		// If the source id is empty and the default destination id is also empty, the
		// matching destination is the same as the default destination.

		// if there is a matching destination entry, and it has the same hash as the
		// source entry (or the entry type is directory)
		//   we are only going to perform a rename on the destination. We can present it
		//   accordingly on the progress bar (near instantaneous operation)

		mdest_num, mdest_exists := dst.ByUuid[s.Uuid]
		mdest := dst.Entries[mdest_num]
		// FIXME: search in destination parent directories

		if s.Uuid == "" && ddest_exists && ddest.Uuid == "" {
			mdest_exists = true
			mdest_num = ddest_num
			mdest = ddest
		}

		_ = mdest
		_ = mdest_exists

		//if mdest_exists && (mdest.Hash == src.Hash || mdest)

		/*

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
		*/
	}
	return success, nil, errs
}
