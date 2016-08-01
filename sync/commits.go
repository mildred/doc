package sync

import (
	"os"
	"path"

	base58 "github.com/jbenet/go-base58"
	"github.com/mildred/doc/commit"
	"github.com/mildred/doc/repo"
)

type CommitPreparatorOpts struct {
}

type CommitPreparator struct {
	PreparatorArgs
	CommitPreparatorOpts

	// Total bytes scanned that can be counted (excluding directories)
	TotalBytes uint64

	// Total items scanned
	NumFiles uint64
}

func (p *CommitPreparatorOpts) Preparator(args *PreparatorArgs) Preparator {
	return &CommitPreparator{
		PreparatorArgs:       *args,
		CommitPreparatorOpts: *p,
	}
}

func (p *CommitPreparator) ScanStatus() (files, bytes uint64) {
	return p.NumFiles, p.TotalBytes
}

func (p *CommitPreparator) PrepareCopy(src, dst string) {
	source_files, err := commit.ReadDirByPath(src)
	if err != nil {
		p.HandleError(err)
		return
	}
	dest_files, err := commit.ReadDirByPath(dst)
	if err != nil {
		p.HandleError(err)
		return
	}

	for file, shash := range source_files {
		dhash, has_dest := dest_files[file]
		p.NumFiles += 1
		if !has_dest {
			srci, err := os.Lstat(path.Join(src, file))
			if err != nil {
				p.HandleError(err)
				continue
			}
			res := p.HandleAction(*NewCopyFile(
				path.Join(src, file),
				path.Join(dst, file),
				base58.Decode(shash),
				srci))
			if !res {
				return
			}
		} else if shash != dhash {
			srci, err := os.Lstat(path.Join(src, file))
			if err != nil {
				p.HandleError(err)
				continue
			}
			dsti, err := os.Lstat(path.Join(dst, file))
			if err != nil {
				p.HandleError(err)
				continue
			}
			dstname := repo.FindConflictFileName(file, base58.Decode(shash))
			if dstname != "" {
				res := p.HandleAction(*NewCopyAction(
					path.Join(src, file),
					path.Join(dst, dstname),
					base58.Decode(shash),
					size(srci),
					path.Base(file),
					true,
					srci.Mode(),
					dsti.Mode()))
				if !res {
					return
				}
			}
		}
	}
}
