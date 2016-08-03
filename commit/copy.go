package commit

import (
	"path/filepath"

	base58 "github.com/jbenet/go-base58"
)

func copyEntries(src []Entry, dst map[string]string) (res []Entry, conflicts []Entry) {
	for _, s := range src {
		h := base58.Encode(s.Hash)
		d, hasd := dst[s.Path]
		if !hasd {
			res = append(res, s)
		} else if d != h {
			conflicts = append(conflicts, s)
		}
	}
	return
}

func parentDirs(entry Entry, ok map[string]bool) []string {
	var res []string
	var breadcrumb []string

	d := filepath.Dir(entry.Path)
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

// Return a list of directories for the entries given.
// Each directory is included once in the result
// The directories are ordered starting from the root directories to the leaves
func Dirs(entries []Entry) []string {
	var res []string
	ok := map[string]bool{}
	ok["."] = true
	ok["/"] = true

	for _, e := range entries {
		d := filepath.Dir(e.Path)
		var breadcrumb []string
		for !ok[d] {
			breadcrumb = append(breadcrumb, d)
			d = filepath.Dir(d)
		}
		for i := len(breadcrumb) - 1; i >= 0; i-- {
			res = append(res, breadcrumb[i])
			ok[breadcrumb[i]] = true
		}
	}
	return res
}
