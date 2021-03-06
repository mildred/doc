package sync

import (
	"fmt"
	"path/filepath"
)

type Logger struct {
	quiet   bool
	verbose bool
	scan    struct {
		src         string
		src_hash    bool
		dst         string
		dst_hash    bool
		num_files   uint64
		total_files uint64
		total_bytes uint64
	}
	copying bool
	exec    struct {
		src   string
		dst   string
		item  uint64
		bytes uint64
	}
	num_errors int
}

func NewLogger(quiet, verbose bool) *Logger {
	return &Logger{quiet: quiet}
}

func (l *Logger) LogPrepare(p Preparator, src, dst string, hash_src, hash_dst bool) {
	l.scan.src = src
	l.scan.src_hash = hash_src
	l.scan.dst = dst
	l.scan.dst_hash = hash_dst
	l.scan.num_files, l.scan.total_bytes = p.ScanStatus()
	l.Print()
}

func (l *Logger) AddFile(act *CopyAction) {
	l.scan.total_files++
	l.Print()
}

func (l *Logger) LogExec(act *CopyAction, bytes uint64, items uint64) {
	l.copying = true
	l.exec.src = act.Src
	l.exec.dst = act.Dst
	l.exec.bytes = bytes
	l.exec.item = items
	l.Print()
}

func (l *Logger) LogError(e error) {
	l.num_errors++
	fmt.Printf("\x1b[K%s\n", e.Error())
	l.Print()
}

func (l *Logger) NumErrors() int {
	return l.num_errors
}

func (l *Logger) Print() {
	if l.quiet {
		return
	}
	fmt.Printf("\x1b[K\n")
	fmt.Printf(
		"\x1b[KScanning: %d files (%d bytes, %d files to copy)\n"+
			"\x1b[K  %s\n",
		l.scan.num_files, l.scan.total_bytes, l.scan.total_files, filepath.Base(l.scan.src))
	if l.scan.src_hash {
		fmt.Printf("\x1b[K  From: %s (hashing)\n", filepath.Dir(l.scan.src))
	} else {
		fmt.Printf("\x1b[K  From: %s\n", filepath.Dir(l.scan.src))
	}
	if l.scan.dst_hash {
		fmt.Printf("\x1b[K  To:   %s (hashing)\n", filepath.Dir(l.scan.dst))
	} else {
		fmt.Printf("\x1b[K  To:   %s\n", filepath.Dir(l.scan.dst))
	}

	if l.copying {
		percentItems := 100.0 * float64(l.exec.item) / float64(l.scan.total_files)
		percentBytes := 100.0 * float64(l.exec.bytes) / float64(l.scan.total_bytes)
		fmt.Printf(
			"\x1b[KCopying: %d bytes %2.0f%% (file %d %2.0f%%)\n"+
				"\x1b[K  %s\n"+
				"\x1b[K  From: %s\n"+
				"\x1b[K  To:   %s\n",
			l.exec.bytes, percentBytes, l.exec.item, percentItems, filepath.Base(l.exec.dst),
			filepath.Dir(l.exec.src),
			filepath.Dir(l.exec.dst))
		fmt.Printf("\x1b[K\x1b[9A")
	} else {
		fmt.Printf("\x1b[K\x1b[5A")
	}
}

func (l *Logger) Clear() {
	if l.quiet {
		return
	}
	fmt.Printf("\n\n\n\n\n")
	if l.copying {
		fmt.Printf("\n\n\n\n")
	}
}
