package copy

import (
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mildred/doc/attrs"
)

// Creates a directory dst from the information found in src
// First error is fatal, other errors are issues replicating attributes
func MkdirFrom(src, dst string) (error, []error) {
	var errs []error

	src_st, err := os.Stat(src)
	if err != nil {
		return err, nil
	}

	err = os.Mkdir(dst, src_st.Mode())
	if err != nil {
		return err, nil
	}

	if stat, ok := src_st.Sys().(*syscall.Stat_t); ok {
		err = os.Lchown(dst, int(stat.Uid), int(stat.Gid))
		if err != nil {
			errs = append(errs, err)
		}

		atime := time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
		err = os.Chtimes(dst, atime, src_st.ModTime())
		if err != nil {
			errs = append(errs, err)
		}
	}

	xattr, values, err := attrs.GetList(src)
	if err != nil {
		errs = append(errs, err)
	}

	for i, attrname := range xattr {
		err = attrs.Set(src, attrname, values[i])
		if err != nil {
			errs = append(errs, err)
		}
	}

	return nil, errs
}

// Create all directrories in dir in dst, using information found from
// directories of the same name in src. The parent directories are assumed to
// exist in dst, if the directories are created in order.
// If a file can be found on dst having the same name, no creation attempt is
// made.
// First errors is fatal, other errors are issues replicating attributes
func Makedirs(dirs []string, src, dst string) (error, []error) {
	var errs_warn []error
	for _, dir := range dirs {
		s := filepath.Join(src, dir)
		d := filepath.Join(dst, dir)
		// Check it doesn't exist on dst
		if _, e := os.Lstat(d); e == nil {
			continue
		}
		// Make from src to dst
		err, erw := MkdirFrom(s, d)
		errs_warn = append(errs_warn, erw...)
		if err != nil {
			return err, errs_warn
		}
	}
	return nil, errs_warn
}
