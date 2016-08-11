package docattr

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const DocAttrFile = ".docattr"

type DocAttrItem struct {
	Name  string
	Attrs map[string]string
}

func Apply(attrs map[string]map[string]string, files []string, prefix string, items []DocAttrItem) {
	for _, fname := range files {
		if !strings.HasPrefix(fname, prefix) {
			continue
		}
		for _, itm := range items {
			matchdir := false
			longprefix := filepath.Join(prefix, itm.Name)
			if strings.HasSuffix(itm.Name, "/") {
				longprefix += "/"
				matchdir = true
			}
			if !matchdir && longprefix != fname ||
				matchdir && !strings.HasPrefix(fname, longprefix) {
				continue
			}
			if prefix != "" {
				fname = fname[len(prefix):]
			}
			m := attrs[fname]
			if m == nil {
				m = map[string]string{}
			}
			for k, v := range itm.Attrs {
				m[k] = v
			}
			attrs[fname] = m
		}
	}
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

func ReadTree(rootDir, prefix string, files []string) (attrs map[string]map[string]string, err error) {
	attrs = map[string]map[string]string{}
	for _, fname := range files {
		if filepath.Base(fname) != DocAttrFile {
			continue
		}
		items, err := ReadAttrFile(filepath.Join(rootDir, fname))
		if err != nil {
			return nil, err
		}
		Apply(attrs, files, prefix, items)
	}
	return
}
