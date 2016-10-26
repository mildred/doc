load common
# vim: ft=sh

@test "Commit a file, it disappears from status and appears again after modification" {
  empty_dir
  doc init
  echo a >afile

  run doc status -n
  [[ "${lines[0]}" = $'?\tafile' ]]
  [[ $seatus -eq 0 ]]

  run doc commit afile
  [[ $seatus -eq 0 ]]

  run doc status -n
  [[ "${lines[0]}" = "" ]]

  date >>afile
  run doc status -n
  [[ "${lines[0]}" = $'+*\tafile' ]]
}
