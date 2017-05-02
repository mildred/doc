
help: ## This help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36mmake %-30s\033[0m %s\n", $$1, $$2}'

test: ## Run tests
	bats/bin/bats tests/*.bats

