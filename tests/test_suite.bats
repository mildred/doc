load common

@test "The doc command exists" {
	doc help
}

@test "Filesystem is writeable" {
	touch test-suite
}

@test "Tests are running in separate dirs" {
	empty_dir
	! test -e test-suite

}

@test "Tests are running in empty dirs" {
	empty_dir
	test -z "$(ls)"
}
