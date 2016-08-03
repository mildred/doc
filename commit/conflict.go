package commit

import (
	"bytes"
	"fmt"
	"path/filepath"
)

// Return a conflict filename to use. Return the empty string if the conflict
// file already exists for the same hash.
func FindConflictFileName(entry Entry, dest_commit *Commit) string {
	hashext := "." + entry.HashText()
	ext := filepath.Ext(entry.Path)
	dstname := fmt.Sprintf("%s%s%s", entry.Path, hashext, ext)
	i := 0
	for {
		idx, exists := dest_commit.ByPath[dstname]
		if exists && bytes.Equal(dest_commit.Entries[idx].Hash, entry.Hash) {
			return ""
		} else if !exists {
			return dstname
		}
		dstname = fmt.Sprintf("%s%s.%d%s", entry.Path, hashext, i, ext)
		i++
	}
}
