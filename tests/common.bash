empty_dir(){
  local d="tests-tmp/${BATS_TEST_FILENAME##*/}-$BATS_TEST_NAME-$BATS_TEST_NUMBER-$RANDOM"
  rm -rf "$d"
  mkdir -p "$d"
  echo "$d"
  cd "$d"
}
