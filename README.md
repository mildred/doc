doc
===

Manages your files and allow easy synchronisation.

You have a directory structure that you share between multiple drives, and you
want to easily synchronize them, in full or in part. doc is designed for that.

It will store in extended attributes next to each file a checksum and detect
when the file is modified. When you copy a directory using doc, it will silently
avoid copying the same files over and over again. Files with identical name but
different content will be copied under another name, and the list of conflicts
will be available for you to double check.

In the future, it will allow you to store PAR2 redundancy information for each
one of your files, allowing you easy recovery in case of disk failures. A check
command is there to verify that file data hasn't been changed.

Note: if you modify a file and prevent the mtime update, the file will be
detected as corrupt.

Roadmap
-------

- Fully deprecating `sync` and `cp` in favor of `push` and `pull`

- `push` and `pull` should commit the destination directory once the operation
  is complete

- conflicts should be easy to locate and resolve (`doc conflicts`, `doc
  resolve`). Should be stored in .doccommit

- push or pull should not propagate a file in conflict, but a log should
  indicate that the file was skipped because it is a conflict. After push or
  pull, a list of conflicts should be shown.

- Directories should be associated a unique id that is the same on all devices.
  The unique id should be stored either in an extended attribute or in a global
  repository indexed by the device and inode.

- When two directories are merged, the new directory should have the two unique
  ids to allow detecting a synchronisation with either one of the sources

- Synchronisation should detect directory renames and replacements using this
  unique id

### Copy algorithm with renames ###

the id must be stored in the extended attributes, and associated with the device
id / inode number. In case the entry is copied, the inode number changes and the
id must be regenerated.

```
loop over all sources entries, including directories (coming before files)

  DEFAULT DESTINATION:
  the entry on the destination located on the same path than on source

  MATCHING DESTINATION:
  if the source has an id, lookup an entry of the same type and id in the
  destination. The search must be done in the destination parent directory as
  This is to prevent to have two different inodes with the same id.
  far as possible (do not filter the .dircommit file on a subdirectory).
  If the source id is empty and the default destination id is alsa empty, the
  matching destination is the same as the default destination.

  if there is a matching destination entry, and it has the same hash as the
  source entry (or the entry type is directory)
    we are only going to perform a rename on the destination. We can present it
    accordingly on the progress bar (near instantaneous operation)

  -- start algorithm --

  if the default destination exists and it matches the id and hash (and,
  optionally, the id is not empty for both):
    (optionally, there is no need to generate an id)
    (there is no id conflict)
    (there is no rename)
    (there is no hash conflict)
    EXIT EARLY, GO TO NEXT FILE

  OPTIONAL if destination exists, and neither source nor destination have an id:
    generate the same id for both, start with source and if it fails, do
    not write an id for destination

  (HANDLE ID CONFLICT)
  if default destination exists and destination or source id is mismatching:
    arrange for the destination file/directory to be renamed because of a
    conflict:
    - rename the destination file (with a timestamp)
      (if new name exists, increase timestamp accuracy, then add serial)
    - mark the destination as a conflict
    - update the destination commit datastructure in memory so the entry can
      still be found
    for the rest of the algorithm, assume the destination does not exists
    (since we renamed the conflicting entry)

  (HANDLE RENAME)
  if the default destination is absent but there is a matching destination
    move the matching destination in place of the default destination
    because the parent entries are handled first, the parent directory of the
    destination will not be moved away

  (HANDLE HASH CONFLICT)
  if the destination exists, it has the same id
    we assume source and destination are the same type (they have ther same id)
    if the type is a directory, assume the content is identical
    else, if the content is different
      rename the destination file and mark a conflict

  (ACTUAL COPY)
  if the destination exists, it must have the same id and hash
  else, if the destination does not exists
    copy the source file to the destination (including the id)
    update the commit structure in memory
```


Usage
-----

### `doc init [DIR]`

Creates a `.dirstore` directory in `DIR` or the current directory. This will be
used to store PAR2 archives and possibly history information about each file (a
checksum list to provide additional information for conflicts and
synchronization of moved files, no yet implemented).

### `doc status [DIR]`

Scan `DIR` or the current directory and display a list of new and modified
files. Conflicts are shown with `C` for the main filename and with `c` for
alternatives.

### `doc check [-a] [DIR]`

Scan `DIR` or the current directory and check for non modified files their
content compared to the stored checksum. If `-a` is specified, modified files
are also shown.

### `doc commit [DIR]`

For each modified file in `DIR` or the current directory, computes a checksum
and store it in the extended attributes.

### `doc cp [SRC] DEST`

Copy each files in `SRC` or the current directory over to `DEST`. Both arguments
are assumed to be directories and `cp` will synchronize from the source to the
destination in the following way:

- Files from source not in the destination: the file is copied
- Files from source existing in the destination with identical content: no
  action is needed
- Files from source existing in the destination with different content: the file
  is copied under a new name, and a conflict is registred with the original file
  in the destination directory.

### `doc save [DIR]`

For each modified file in `DIR` or the current directory, computes a checksum
and store it in the extended attributes. A PAR2 archive is also created and
stored separately in the `.dirstore` directory.

### `doc sync [DIR1] DIR2`

Same as `cp` but the synchronisation is bidirectional. `sync` takes care not to
copy over and over the conflict files.

Future Usage
------------

- `doc commit` shall place in the .dirstore an index file containing for each
  committed file, its timestamp and hash. More quickly parsed than the
  filesystem inodes, it can be used to compare two dirstores for missing files

  alternatively, doc commit shall create an index file named `.dircommit` in the
  committed directory containing the same content as `doc status -c` with the
  modification time

### `doc missing -from SRC [DEST]`, `doc missing -to DEST [SRC]`, `doc missing [SRC] DEST`

Show missing files in SRC that are in DEST

SRC and DEST can be files generated by `doc status -c`

### `doc diff -from SRC [DEST]`, `doc diff -to DEST [SRC]`, `doc diff [SRC] DEST`

Show differences between SRC and DEST. Missing files in DEST are marked with -
and missing files in SRC are marked +.

SRC and DEST can be files generated by `doc status -c`

### `doc sync -from SRC [DEST]`, `doc sync -to DEST [SRC]`

Same as `doc cp SRC DEST`, but for each new file copied from `SRC`, duplicates
are searched in `DEST` and removed. The typical use case is when the `SRC` copy
contains moved files. Those will also be moved in the `DEST` copy (provided they
haven't changed).

### `doc resolve [-rm] FILE`

Mark the `FILE` as resolved using its current content. Alternatives are removed
only if `-rm` is specified, otherwise they loose their link with the original
file.

### `doc restore -a|FILE`

Restore `FILE` or all corrupted files if `-a` is spcified using the PAR2
information.

### `doc prune n|-f [DIR]`

Prune old PAR2 archives from `.dirstore` in `DIR` or the current directory.
`.dirstore` must be a direct descendent of `DIR`.

Installation
============

Installation is the same as for any go package. After you installed the go
language tools, you have to export the `GOPATH` nvironment variable to an empty
directory you have created for the occasion:

        mkdir -p ~/Projects/go
        echo 'export GOPATH="$HOME/Projects/go"' >> ~/.profile
        export GOPATH="$HOME/Projects/go"

Then, you gan get, build and install this package:

        go get -u github.com/mildred/doc
        go build github.com/mildred/doc
        go install github.com/mildred/doc

The resulting binary is installed in `GOPATH/bin`. You have to add it to your
`PATH`:

        echo 'export PATH="$PATH:$HOME/Projects/go/bin"' >> ~/.profile
        export PATH="$PATH:$HOME/Projects/go/bin"

You should then be able to run `doc`:

        doc help

Bugs
====

- cp, sync: Traversing symlinks without cycle detection

- cp, sync: Shold not perform a Stat() on the files but a Lstat() to avoid
  following symlinks.

- cp, sync: have a continuous mode where scanning is performed in a goroutine
  that will send action to aother goroutine that will do the actual copy. Have a
  multi-line status that updates itself like:

        Scan: 94369 files scanned, 128746389 bytes
              path/trunc/to/80/char
              very-long-filename...last-10-chars.jpeg
        Copy: [======>           ] 34%
              884/8991 files, 78954/789563 bytes
              path/being/copied
              filename

  Truncate filenames by keeping the first characters, the ellipsis and the last
  10 characters (to include the extension). Pathnames should be truncated so
  they would fit a 80 character line, each path item should be truncated
  identically.

  The sequential mode should still be kept there.

- cp, sync: avoid recreating conflicts over and over for the execution of the
  same command multiple times
