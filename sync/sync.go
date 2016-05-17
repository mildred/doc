package sync

import (
	"fmt"
	"os"
)

type Preparator interface {
	PrepareCopy(src, dst string)
	ScanStatus() (files, bytes uint64)
}

type PreparatorArgs struct {
	// Gather dediplication data if not nil: for each hash (as keys, binary hash:
	// not digests), store a list of matching paths (for the destination directory
	// only).
	Dedup map[string][]string

	// [MANDATORY] Called when an action is to be taken. Should return true. False
	// is used to stop scanning
	HandleAction func(act CopyAction) bool

	// [MANDATORY] Called when an error happens. Should return true to continue
	// scanning or false to stop scaning.
	HandleError func(e error) bool

	// Logger to be called for each scanned item
	// Always called with hashing to false. When performing hashing, it is called
	// a second or a third time with either hash_src or hash_dst set to true,
	// depending on which file is being hashed.
	Logger func(p Preparator, src, dst string, hash_src, hash_dst bool)
}

type PreparatorOptions interface {
	Preparator(args *PreparatorArgs) Preparator
}

type SyncOptions struct {
	// Preparator, if nil, DefaultPreparator is used
	Preparator PreparatorOptions

	// Don't execute synchronisation, just print what it would have done
	DryRun bool

	// Force operation even after first error
	Force bool

	// If true, do not print anything except errors
	Quiet bool

	// If true, try to hard link files from within the destination directory if
	// possible instead of copying them from the source directory.
	Dedup bool

	// Delete duplicates in destination that are not in source
	DeleteDup bool

	// Scan first and copy after scanning is completed only.
	TwoPass bool
}

func Sync(src, dst string, opt SyncOptions) (numErrors int) {
	var dedup_map map[string][]string = nil
	if opt.Dedup {
		dedup_map = map[string][]string{}
	}

	if !opt.Quiet {
		fmt.Printf("Source:      %s\n", src)
		fmt.Printf("Destination: %s\n", dst)
	}

	logger := NewLogger(opt.Quiet)

	var actions_chan chan *CopyAction = make(chan *CopyAction, 100)
	var actions_slice []*CopyAction
	var actions_closed bool = false

	prep := opt.Preparator.Preparator(&PreparatorArgs{
		Dedup:  dedup_map,
		Logger: logger.LogPrepare,
		HandleError: func(e error) bool {
			logger.LogError(e)
			if !opt.Force && !opt.DryRun {
				if !opt.TwoPass {
					close(actions_chan)
					actions_closed = true
				}
				return false
			}
			return true
		},
		HandleAction: func(act CopyAction) bool {
			logger.AddFile(&act)
			if opt.TwoPass {
				actions_slice = append(actions_slice, &act)
				return true
			} else {
				if !actions_closed {
					actions_chan <- &act
					return true
				}
				return false
			}
		},
	})

	defer logger.Clear()

	if opt.TwoPass {
		prep.PrepareCopy(src, dst)

		if logger.NumErrors() > 0 && !opt.Force && !opt.DryRun {
			fmt.Println("Stopping because of errors")
			return logger.NumErrors()
		}

		go func() {
			for _, act := range actions_slice {
				actions_chan <- act
			}
			close(actions_chan)
			actions_closed = true
		}()
	} else {
		go func() {
			prep.PrepareCopy(src, dst)
			close(actions_chan)
		}()
	}

	exec := &Executor{
		DryRun:    opt.DryRun,
		Force:     opt.Force,
		Dedup:     dedup_map,
		LogAction: logger.LogExec,
		LogError:  logger.LogError,
	}

	conflicts, dup_hashes := exec.Execute(actions_chan)

	if opt.DeleteDup {
		for _, h := range dup_hashes {
			for _, path := range dedup_map[string(h)] {
				if opt.DryRun {
					fmt.Sprintf("rm -f %s\n", path)
				} else {
					err := os.Remove(path)
					if err != nil {
						logger.LogError(fmt.Errorf("remove %s: %s", path, err.Error()))
					}
				}
			}
		}
	}

	for _, c := range conflicts {
		fmt.Fprintf(os.Stderr, "CONFLICT %s\n", c)
	}

	return logger.NumErrors()
}
