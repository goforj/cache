ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
$(eval $(ARGS):;@:)


HELP_FUN = %help; while (<>) { /^([A-Za-z0-9_-]+)\s*:.*\#\#(?:@([A-Za-z0-9_-]+))?\s(.*)$$/ or next; push @{$$help{$$2 || "other"}}, [$$1, $$3]; $$width = length($$1) if length($$1) > $$width } print "\e[1;97m$(or $(HELP_NAME),$(notdir $(CURDIR)))\e[0m\n\n"; for $$category (sort keys %help) { print "\e[1;97m$$category\e[0m\n"; for $$entry (@{$$help{$$category}}) { printf "  \e[1;32m%-*s\e[0m  \e[90m%s\e[0m\n", $$width, $$entry->[0], $$entry->[1] } }

help: ##@other Show this help.
	@perl -e '$(HELP_FUN)' $(MAKEFILE_LIST)

##@tests
test: ##@tests Run the test suite.
	go test ./...

##@analysis
vet: ##@analysis Run Go vet.
	go vet ./...

##@documentation
generate: ##@documentation Regenerate the documentation.
	go -C docs run ./readme

docs-watch: ##@documentation Watch source changes and regenerate documentation.
	sh docs/watcher.sh

test-integration: ##@tests Run integration tests: make test-integration [all|null|file|memory|dynamocache|rediscache|memcachedcache|natscache|sqlitecache|postgrescache|mysqlcache].
	INTEGRATION_DRIVER="$(or $(firstword $(ARGS)),all)" go -C integration test -tags=integration ./...
