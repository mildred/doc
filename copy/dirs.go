package copy

import (
	"os"
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
