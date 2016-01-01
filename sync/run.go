package sync

type Executor struct {
  // Dry run: only show actions
  DryRun bool

  // Force operation even after first error
  Force bool

  // If not nil, this is a map that associate a list of files for each hash (in
  // binary form). This is used to hard link from those files instead of copying
  // from the source directory. if nil, deduplication is desactivated.
  Dedup map[string][]string

  // Called to log an action (both dry mode and normal mode)
  LogAction func(act *CopyAction, bytes uint64, items uint64)

  // Called when there is an error
  LogError func(e error)
}

func (e *Executor) Execute(actions <-chan *CopyAction) (conflicts []string, duplicate_hashes [][]byte) {
  var execBytes uint64 = 0
  var numFiles uint64 = 0

  for act := range actions {
    numFiles++
    if act.Conflict {
      conflicts = append(conflicts, act.Dst)
    }
    if e.Dedup != nil && act.Dsthash != nil {
      if files, ok := e.Dedup[string(act.Dsthash)]; ok && len(files) > 0 {
        duplicate_hashes = append(duplicate_hashes, act.Dsthash)
        act.Src = files[0]
        act.Link = true
      }
    }
    if e.LogAction != nil && numFiles == 1 {
      e.LogAction(act, execBytes, 0)
    }
    if ! e.DryRun {
      err := act.Run()
      execBytes += uint64(act.Size)
      if err != nil {
        e.LogError(err)
        if ! e.Force {
          break
        }
        continue
      }
    }
    if e.LogAction != nil {
      e.LogAction(act, execBytes, uint64(numFiles))
    }
  }
  return
}

