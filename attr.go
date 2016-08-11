package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mildred/doc/commit"
)

const usageAttr string = `doc attr [OPTIONS...] [DIR]

Show attributes applied to DIR or the current directory. Attributes are
specified in files named .docattr and are valid for filesi nthe same directory
and below. The syntax is:

    | /FILENAME
    |     attr=value
    |     attr=value
    | /FILENAME
    |     attr=value
    | ...

If a filename ends with '/', then the attributes are recursive to files in that
directory.

Options:
`

func mainAttr(args []string) int {
	f := flag.NewFlagSet("status", flag.ExitOnError)
	f.Usage = func() {
		fmt.Print(usageAttr)
		f.PrintDefaults()
	}
	f.Parse(args)
	dir := f.Arg(0)
	if dir == "" {
		dir = "."
	}

	c, err := commit.ReadCommit(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	for f, a := range c.Attrs {
		path := filepath.Join(dir, f)
		if strings.HasSuffix(f, "/") {
			path = path + "/"
		}
		if path == "./" {
			path = ""
		}
		fmt.Printf("/%s\n", path)
		for k, v := range a {
			fmt.Printf(" %s=%s\n", k, v)
		}
	}

	return 0
}
