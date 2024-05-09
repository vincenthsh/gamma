default: build

build: ## Build binary
	go build -o bin/gamma

# install goreleaser with
# brew install goreleaser/tap/goreleaser
snapshot: ## Build snapshot using goreleaser (requires goreleaser to be installed)
	goreleaser build --snapshot --clean

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
