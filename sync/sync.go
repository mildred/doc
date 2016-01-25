package sync

import (
	"fmt"
	"os"
)

type SyncOptions struct {
	// Don't execute synchronisation, just print what it would have done
	DryRun bool

	// Force operation even after first error
	Force bool

	// If true, do not print anything except errors
	Quiet bool

	// If true, commit new hash for out of date files
	Commit bool

	// If true, try to hard link files from within the destination directory if
	// possible instead of copying them from the source directory.
	Dedup bool

	// Delete duplicates in destination that are not in source
	DeleteDup bool

	// If true, scan files that are out of date to get the new hash
	CheckHash bool

	// Bidirectional scan: return actions to synchronize both from source to
	// destination and from destination to source.
	Bidir bool

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

	prep := &Preparator{
		CheckHash: opt.CheckHash,
		Bidir:     opt.Bidir,
		Commit:    opt.Commit,
		Dedup:     dedup_map,
		Logger:    logger.LogPrepare,
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
	}

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
