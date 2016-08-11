package docattr

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const DocAttrFile = ".docattr"

type DocAttrItem struct {
	Name  string
	Attrs map[string]string
}

func ReadAttrFile(path string) (items []DocAttrItem, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if len(items) > 0 {
				elems := strings.SplitN(strings.TrimLeft(line, " \t"), "=", 2)
				key := elems[0]
				val := ""
				if len(elems) > 1 {
					val = elems[1]
				}
				items[len(items)-1].Attrs[key] = val
			}
		} else {
			items = append(items, DocAttrItem{line, map[string]string{}})
		}
	}
	err = scanner.Err()
	return
}

type bySize []string

func (a bySize) Len() int           { return len(a) }
func (a bySize) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a bySize) Less(i, j int) bool { return len(a[i]) < len(a[j]) }

func ReadTree(rootDir, prefix string, files []string) (attrs map[string]map[string]string, err error) {
	var filenames []string
	attrFiles := map[string][]DocAttrItem{}
	attrs = map[string]map[string]string{}

	for _, fname := range files {
		if filepath.Base(fname) != DocAttrFile {
			continue
		}
		dir := filepath.Dir(fname) + "/"
		if len(prefix) <= len(dir) && !strings.HasPrefix(dir, prefix) {
			continue
		}
		filenames = append(filenames, fname)
		attrFiles[fname], err = ReadAttrFile(filepath.Join(rootDir, fname))
		if err != nil {
			return nil, err
		}
	}

	sort.Sort(bySize(filenames))
	for _, fname := range filenames {
		dir := filepath.Dir(fname)
		for _, itm := range attrFiles[fname] {
			name := filepath.Join(dir, itm.Name)
			if strings.HasSuffix(itm.Name, "/") {
				name = name + "/"
			}
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			name = name[len(prefix):]
			m := attrs[name]
			if m == nil {
				m = map[string]string{}
			}
			for k, v := range itm.Attrs {
				m[k] = v
			}
			attrs[name] = m
		}
	}
	return
}
