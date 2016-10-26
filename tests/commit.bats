load common
# vim: ft=sh

@test "Commit a file, it disappears from status and appears again after modification" {
  empty_dir
  doc init
  echo a >afile

  run sh -c 'doc status -n | grep -v doccommit'
  [[ "${lines[0]}" = $'?\tafile' ]]
  [[ $seatus -eq 0 ]]

  run doc commit afile
  [[ $seatus -eq 0 ]]

  run sh -c 'run doc status -n | grep -v doccommit'
  [[ "${lines[0]}" = "" ]]

  date >>afile
  run sh -c 'run doc status -n | grep -v doccommit'
  [[ "${lines[0]}" = $'+*\tafile' ]]
}
