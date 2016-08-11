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
	if err != nil && !os.IsExist(err) {
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

func makeParentDirs(srcdir, dstdir, path string, okdirs map[string]bool) (error, []error) {
	var errs []error
	for _, dir := range parentDirs(path, okdirs) {
		err, ers := MkdirFrom(filepath.Join(srcdir, dir), filepath.Join(dstdir, dir))
		errs = append(errs, ers...)
		if err != nil {
			return err, errs
		}
		okdirs[dir] = true
	}
	return nil, errs
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
