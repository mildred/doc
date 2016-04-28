package sync

import (
	"bytes"
	"fmt"
	"github.com/mildred/doc/ignore"
	"github.com/mildred/doc/repo"
	"os"
	"path/filepath"
)

type FilePreparatorOpts struct {
	// If true, scan files that are out of date to get the new hash
	CheckHash bool

	// Bidirectional scan: return actions to synchronize both from source to
	// destination and from destination to source.
	Bidir bool

	// If true, commit new hash for out of date files
	Commit bool

	// Respect .docignore files
	DocIgnore bool
}

type FilePreparator struct {
	PreparatorArgs
	FilePreparatorOpts

	// Verbose
	Verbose bool

	// Total bytes scanned that can be counted (excluding directories)
	TotalBytes uint64

	// Total items scanned
	NumFiles uint64

	// for Log function
	hashingMsg bool
}

func (p *FilePreparatorOpts) Preparator(args *PreparatorArgs) Preparator {
	return &FilePreparator{
		PreparatorArgs:     *args,
		FilePreparatorOpts: *p,
	}
}

func (p *FilePreparator) ScanStatus() (files, bytes uint64) {
	return p.NumFiles, p.TotalBytes
}

func size(info os.FileInfo) int64 {
	if info.Mode()&os.ModeSymlink != 0 {
		return 0
	}
	// FIXME: size of directories
	return info.Size()
}

func (p *FilePreparator) PrepareCopy(src, dst string) {
	p.prepareCopy(src, dst)
}

func (p *FilePreparator) prepareCopy(src, dst string) bool {
	var err error

	if p.Logger != nil {
		p.Logger(p, src, dst, false, false)
	}
	p.NumFiles += 1

	srci, srcerr := os.Lstat(src)
	dsti, dsterr := os.Lstat(dst)

	//
	// File in source but not in destination
	//

	if os.IsNotExist(dsterr) && srcerr == nil {

		if srci.IsDir() {

			if p.DocIgnore && ignore.IsIgnored(src) {
				if p.Verbose {
					fmt.Printf("Ignoring %s\n", src)
				}
				return true
			}

			res := p.HandleAction(*NewCreateDir(src, dst, srci))
			if !res {
				return false
			}

			f, err := os.Open(src)
			if err != nil {
				return p.HandleError(err)
			}
			defer f.Close()

			names, err := f.Readdirnames(-1)
			if err != nil {
				return p.HandleError(err)
			}

			for _, name := range names {
				if !p.prepareCopy(filepath.Join(src, name), filepath.Join(dst, name)) {
					return false
				}
			}

			return true

		} else {

			var srchash []byte

			if !srci.IsDir() {
				srchash, err = repo.GetHash(src, srci, p.Dedup != nil)
				if err != nil {
					return p.HandleError(err)
				}
			}

			res := p.HandleAction(*NewCopyFile(src, dst, srchash, srci))
			p.TotalBytes += uint64(size(srci))
			return res
		}

	}

	//
	// [Bidir] File in destination but not in source: reverse copy
	//

	if p.Bidir && os.IsNotExist(srcerr) && dsterr == nil {

		// FIXME: this could probably be simplified into
		// return p.prepareCopy(dst, src)

		if dsti.IsDir() {

			if p.DocIgnore && ignore.IsIgnored(dst) {
				if p.Verbose {
					fmt.Printf("Ignoring %s\n", dst)
				}
				return true
			}

			res := p.HandleAction(*NewCreateDir(dst, src, dsti))
			if !res {
				return false
			}

			f, err := os.Open(dst)
			if err != nil {
				return p.HandleError(err)
			}
			defer f.Close()

			names, err := f.Readdirnames(-1)
			if err != nil {
				return p.HandleError(err)
			}

			for _, name := range names {
				if !p.prepareCopy(filepath.Join(src, name), filepath.Join(dst, name)) {
					return false
				}
			}

			return true

		} else {

			var dsthash []byte
			if !dsti.IsDir() {
				dsthash, err = repo.GetHash(dst, dsti, p.Dedup != nil)
				if err != nil {
					return p.HandleError(err)
				}
			}

			res := p.HandleAction(*NewCopyAction(dst, src, dsthash, size(dsti), "", false, dsti.Mode(), 0))
			p.TotalBytes += uint64(size(dsti))
			return res
		}
	}

	//
	// [Dedup] File in destination but not in source: register in dedup
	//

	if p.Dedup != nil && os.IsNotExist(srcerr) && dsterr == nil {

		hash, err := repo.GetHash(dst, dsti, p.CheckHash)
		if err != nil {
			if !p.HandleError(err) {
				return false
			}
		} else if hash != nil {
			p.Dedup[string(hash)] = append(p.Dedup[string(hash)], dst)
		}
	}

	//
	// Handle stat() errors
	//

	if srcerr != nil {
		return p.HandleError(srcerr)
	}

	if dsterr != nil {
		return p.HandleError(dsterr)
	}

	//
	// Both source and destination are directories, merge
	//

	if srci.IsDir() && dsti.IsDir() {

		if p.DocIgnore && (ignore.IsIgnored(src) || ignore.IsIgnored(dst)) {
			if p.Verbose {
				fmt.Printf("Ignoring %s (source and destination)\n", src)
			}
			return true
		}

		var srcnames map[string]bool

		if p.Bidir {
			srcnames = map[string]bool{}
		}

		f, err := os.Open(src)
		if err != nil {
			return p.HandleError(err)
		}
		defer f.Close()
		names, err := f.Readdirnames(-1)
		if err != nil {
			return p.HandleError(err)
		}

		for _, name := range names {
			if p.Bidir {
				srcnames[name] = true
			}
			if !p.prepareCopy(filepath.Join(src, name), filepath.Join(dst, name)) {
				return false
			}
		}

		if p.Bidir {

			f, err := os.Open(dst)
			if err != nil {
				return p.HandleError(err)
			}
			defer f.Close()
			dstnames, err := f.Readdirnames(-1)
			if err != nil {
				return p.HandleError(err)
			}

			for _, name := range dstnames {
				if srcnames[name] {
					continue
				}
				if !p.prepareCopy(filepath.Join(src, name), filepath.Join(dst, name)) {
					return false
				}
			}

		}

		return true

	}

	//
	// Source and destination are regular files
	// If hash is different, there is a conflict
	//

	var srch, dsth []byte
	if !srci.IsDir() {
		srch, err = repo.GetHash(src, srci, false)
		computed := false
		if err == nil && srch == nil {
			if p.Logger != nil {
				p.Logger(p, src, dst, true, false)
			}
			srch, err = repo.HashFile(src, srci)
			computed = true
		}
		if err == nil && computed && p.Commit {
			_, err = repo.CommitFileHash(src, srci, srch, false)
		}
		if err != nil {
			return p.HandleError(err)
		}
	}
	if !dsti.IsDir() {
		dsth, err = repo.GetHash(dst, dsti, false)
		computed := false
		if err == nil && dsth == nil {
			if p.Logger != nil {
				p.Logger(p, src, dst, false, true)
			}
			dsth, err = repo.HashFile(dst, dsti)
			computed = true
		}
		if err == nil && computed && p.Commit {
			_, err = repo.CommitFileHash(dst, dsti, dsth, false)
		}
		if err != nil {
			return p.HandleError(err)
		}
	}
	if bytes.Equal(srch, dsth) && srci.Mode()&os.ModeSymlink == dsti.Mode()&os.ModeSymlink {
		return true
	}

	if repo.ConflictFile(src) == "" {
		dstname := repo.FindConflictFileName(dst, srch)
		if dstname != "" {
			p.TotalBytes += uint64(size(srci))
			if !p.HandleAction(*NewCopyAction(src, dstname, srch, size(srci), dst, true, srci.Mode(), dsti.Mode())) {
				return false
			}
		}
	}

	if p.Bidir && repo.ConflictFile(dst) == "" {
		srcname := repo.FindConflictFileName(src, dsth)
		if srcname != "" {
			p.TotalBytes += uint64(size(dsti))
			if !p.HandleAction(*NewCopyAction(dst, srcname, dsth, size(dsti), src, true, dsti.Mode(), srci.Mode())) {
				return false
			}
		}
	}

	return true
}
