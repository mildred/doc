package main

import (
	"flag"
	"fmt"
	"os"

	sync "github.com/mildred/doc/sync"
)

const syncUsage string = `doc sync [OPTIONS...] [SRC] DEST
doc sync [OPTIONS...] -from SRC [DEST]
doc sync [OPTIONS...] -to DEST [SRC]

Copy each files in SRC or the current directory over to DEST, and each of DEST
over to SRC. Both arguments are assumed to be directories and the
synchronisation will be according to the following rules:

  *     Files from source not in the destination: the file is copied
 
  *     Files not in source but in destination: the file is copied
 
  *     Files from source existing in the destination with identical content: no
        action is needed
 
  *     Files from source existing in the destination with different content:
        the file is copied under a new name in both directions (the original
        files are kept) and a conflict is registered with the original files.

Unless the force flag is specified, the operation will stop on the first error.

The operatios is performed in two steps. The first step collects information
about each file and deduce the action to perform, and the second step performs
the actual copy. Interrupting the process during its first step leave your
filesystem untouched. During this parsing step, if files are out of date, their
hash will be computed and that can introduce a delay.

Options:
`

const copyUsage string = `doc cp [OPTIONS...] [SRC] DEST
doc cp [OPTIONS...] -from SRC [DEST]
doc cp [OPTIONS...] -to DEST [SRC]

Copy each files in SRC or the current directory over to DEST. Both arguments are
assumed to be directories and cp will synchronize from the source to the
destination in the following way:

  *     Files from source not in the destination: the file is copied
 
  *     Files from source existing in the destination with identical content: no
        action is needed
 
  *     Files from source existing in the destination with different content:
        the file is copied under a new name, and a conflict is registred with
        the original file in the destination directory.

Unless the force flag is specified, the operation will stop on the first error.

The operatios is performed in two steps. The first step collects information
about each file and deduce the action to perform, and the second step performs
the actual copy. Interrupting the process during its first step leave your
filesystem untouched. During this parsing step, if files are out of date, their
hash will be computed and that can introduce a delay.

Options:
`

func mainCopy(args []string) int {
	f := flag.NewFlagSet("cp", flag.ExitOnError)
	opt_dry_run := f.Bool("n", false, "Dry run")
	opt_quiet := f.Bool("q", false, "Quiet")
	opt_force := f.Bool("f", false, "Force copy even if there are errors")
	opt_hash := f.Bool("c", false, "If necessary, check real hash when deduplicating")
	opt_dedup := f.Bool("d", false, "Deduplicate files on destination (link files on destination instead of copying them from source if possible)")
	opt_dd := f.Bool("dd", false, "Like -d but also remove duplicate files")
	opt_from := f.String("from", "", "Specify the source directory")
	opt_to := f.String("to", "", "Specify the destination directory")
	opt_commit := f.Bool("commit", false, "Commit the new hash if it has been computed")
	opt_2pass := f.Bool("2", false, "Scan before copy in two distinct pass")
	f.Usage = func() {
		fmt.Print(copyUsage)
		f.PrintDefaults()
	}
	f.Parse(args)

	src, dst := findSourceDest(*opt_from, *opt_to, f.Args())
	sync_opts := sync.SyncOptions{
		DryRun:    *opt_dry_run,
		Force:     *opt_force,
		Quiet:     *opt_quiet,
		Commit:    *opt_commit,
		Dedup:     *opt_dedup || *opt_dd,
		DeleteDup: *opt_dd,
		CheckHash: *opt_hash,
		Bidir:     false,
		TwoPass:   *opt_2pass,
	}
	if sync.Sync(src, dst, sync_opts) > 0 {
		os.Exit(1)
	}
	return 0
}

func mainSync(args []string) int {
	f := flag.NewFlagSet("sync", flag.ExitOnError)
	opt_dry_run := f.Bool("n", false, "Dry run")
	opt_quiet := f.Bool("q", false, "Quiet")
	opt_force := f.Bool("f", false, "Force copy even if there are errors")
	opt_from := f.String("from", "", "Specify the source directory")
	opt_to := f.String("to", "", "Specify the destination directory")
	opt_commit := f.Bool("commit", false, "Commit the new hash if it has been computed")
	opt_2pass := f.Bool("2", false, "Scan before copy in two distinct pass")
	f.Usage = func() {
		fmt.Print(syncUsage)
		f.PrintDefaults()
	}
	f.Parse(args)

	src, dst := findSourceDest(*opt_from, *opt_to, f.Args())
	sync_opts := sync.SyncOptions{
		DryRun:    *opt_dry_run,
		Force:     *opt_force,
		Quiet:     *opt_quiet,
		Commit:    *opt_commit,
		Dedup:     false,
		DeleteDup: false,
		CheckHash: false,
		Bidir:     true,
		TwoPass:   *opt_2pass,
	}
	if sync.Sync(src, dst, sync_opts) > 0 {
		os.Exit(1)
	}
	return 0
}
