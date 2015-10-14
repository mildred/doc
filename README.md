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

Usage
-----

### `doc init [DIR]`

Creates a `.dirstore` directory in `DIR` or the current directory. This will be
used to store PAR2 archives and possibly history information about each file.

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

Future Usage
------------

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

### `doc sync [DIR1] DIR2`

Same as `cp` but the synchronisation is bidirectional. `sync` takes care not to
copy over and over the conflict files.

### `doc resolve [-rm] FILE`

Mark the `FILE` as resolved using its current content. Alternatives are removed
only if `-rm` is specified, otherwise they loose their link with the original
file.

### `doc restore -a|FILE`

Restore `FILE` or all corrupted files if `-a` is spcified using the PAR2
information.

### `doc save [DIR]`

For each modified file in `DIR` or the current directory, computes a checksum
and store it in the extended attributes. A PAR2 archive is also created and
stored separately in the `.dirstore` directory.

### `doc prune n|-f [DIR]`

Prune old PAR2 archives from `.dirstore` in `DIR` or the current directory.
`.dirstore` must be a direct descendent of `DIR`.

