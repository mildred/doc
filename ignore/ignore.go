package ignore

import (
	"os"
	"path/filepath"
)

func IsIgnored(path string) bool {
	if filepath.Base(path) == ".dirstore" {
		return true
	}
	st, err := os.Lstat(filepath.Join(path, ".docignore"))
	if err == nil && st.Size() == 0 {
		return true
	}
	return false
}
