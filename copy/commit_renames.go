package copy

import (
	"fmt"

	"github.com/mildred/doc/commit"
)

func findDefaultDestination(src, dst *commit.Commit, s Entry) (num int, exists bool, ddest Entry) {
	// DEFAULT DESTINATION:
	// the entry on the destination located on the same path than on source

	num, exists = dst.ByPath[s.Path]
	if exists {
		ddest = dst.Entries[num]
	}
}

func findMatchingDestination(src, dst *commit.Commit, s Entry) (mdest_num int, mdest_exists bool, mdest Entry) {
	// MATCHING DESTINATION:
	// if the source has an id, lookup an entry of the same type and id in the
	// destination. The search must be done in the destination parent directory as
	// far as possible (do not filter the .dircommit file on a subdirectory).
	// This is to prevent to have two different inodes with the same id.

	// If the source id is empty and the default destination id is also empty, the
	// matching destination is the same as the default destination.

	_, ddest_exists, ddest = findDefaultDestination(src, dst, s)

	if s.Uuid != "" {
		mdest_num, mdest_exists = dst.ByUuid[s.Uuid]
		// FIXME: search in destination parent directories
	}

	if s.Uuid == "" && ddest_exists && ddest.Uuid == "" {
		mdest_exists = ddest_exists
		mdest_num = ddest_num
	}

	if mdest_exists {
		mdest = dst.Entryes[e.ddest_num]
	}
}

func copyTreeRename(srcdir, dstdir string, src, dst *commit.Commit, p Progress) ([]commit.Entry, error, []error) {
	var errs []error
	var success []commit.Entry
	var staticsteps = 3
	var numfiles = 1

	if p != nil {
		p.SetProgress(2, numfiles+staticsteps, "Prepare copy: open "+dstdir)
	}

	c, err := commit.OpenDirAppend(dstdir)
	if err != nil {
		return success, err, errs
	}
	defer c.Close()

	if p != nil {
		p.SetProgress(2, numfiles+staticsteps, "Prepare copy: compute how many files to copy")
	}

	numfiles := 0
	for _, s := range src.Entries {
		// if there is a matching destination entry, and it has the same hash as the
		// source entry (or the entry type is directory)
		//   we are only going to perform a rename on the destination. We can present it
		//   accordingly on the progress bar (near instantaneous operation)

		_, mdest_exists, mdest = findMatchingDestination(src, dst, s)

		if !(mdest_exists && (mdest.Uuid == s.Uuid || strings.HasSuffix(mdest.Path, "/"))) {
			numfiles = numfiles + 1
		}
	}

	if p != nil {
		p.SetProgress(2, numfiles+staticsteps, fmt.Sprintf("Prepare copy: starting copy for %d files...", numfiles))
	}

	// loop over all sources entries, including directories (coming before files)

	curprogress := 3
	for i, s := range src.Entries {
		_, ddest_exists, ddest = findDefaultDestination(src, dst, s)
		_, mdest_exists, mdest = findMatchingDestination(src, dst, s)

		if mdest_exists && (mdest.Uuid == s.Uuid || strings.HasSuffix(mdest.Path, "/")) {
			curprogress = curprogress + 1
		}

		// if the default destination exists and it matches the id and hash (and,
		// optionally, the id is not empty for both):
		//   (optionally, there is no need to generate an id)
		//   (there is no id conflict)
		//   (there is no rename)
		//   (there is no hash conflict)
		//   EXIT EARLY, GO TO NEXT FILE

		if ddest_exists && ddest.Uuid == s.Uuid && ddest.Hash == s.Hash && s.Uuid != "" {
			continue
		}

		if p != nil {
			p.SetProgress(curprogress, numfiles+staticsteps, fmt.Sprintf("%s", s.Path))
		}

		// OPTIONAL if destination exists, and neither source nor destination have
		// an id:
		//   generate the same id for both, start with source and if it fails, do
		//   not write an id for destination

		if ddest_exists && s.Uuid == "" && ddest.Uuid == "" {
			// FIXME: generate an id
		}

		// (HANDLE ID CONFLICT)
		// if default destination exists and destination or source id is mismatching:
		//   arrange for the destination file/directory to be renamed because of a
		//   conflict:
		//   - rename the destination file (with a timestamp)
		//     (if new name exists, increase timestamp accuracy, then add serial)
		//   - mark the destination as a conflict
		//   - update the destination commit datastructure in memory so the entry
		//     can still be found
		//   for the rest of the algorithm, assume the destination does not exists
		//   (since we renamed the conflicting entry)

		if ddest_exists && ddest.Uuid != s.Uuid {
			// FIXME: rename ddest
			ddest_exists = false
		}

		// (HANDLE RENAME)
		// if the default destination is absent but there is a matching destination
		//   move the matching destination in place of the default destination
		//   because the parent entries are handled first, the parent directory of
		//   the destination will not be moved away

		if !ddest_exists && mdest_exists {
			// FIXME: rename mdest to ddest
			ddest = mdest
			ddest_exists = true
		}

		// (HANDLE HASH CONFLICT)
		// if the destination exists, it has the same id
		//   we assume source and destination are the same type (they have ther same
		//   id)
		//   if the type is a directory, assume the content is identical
		//   else, if the content is different
		//     rename the destination file and mark a conflict

		if ddest_exists && ddest.Uuid != s.Uuid {
			panic(fmt.Sprintf("source %v and destination %v have mismatching UUID %v %v", s.Path, ddest.Path, s.Uuid, ddest.Uuid))
		}

		if ddest_exists && !strings.HasSuffix(s.Path, "/") && ddest.Hash != s.Hash {
			// FIXME: rename ddest to conflict file and mark conflict
			ddest_exists = false
		}

		// (ACTUAL COPY)
		// if the destination exists, it must have the same id and hash
		// else, if the destination does not exists
		//   copy the source file to the destination (including the id)
		//   update the commit structure in memory

		if ddest_exists && ddest.Hash != s.Hash {
			panic(fmt.Sprintf("source %v and destination %v have mismatching hash %v %v", s.Path, ddest.Path, s.Hash, ddest.Hash))
		}

		if !ddest_exists {
			// FIXME: copy source file to destination (copy with UUID, generate UUID
			// if necessary)
		}

		// FIXME: update commit structure in memory
	}
	return success, nil, errs
}
