.PHONY: fixtures test build

# Regenerate the bundled commit-chain diffs from the engine (see script header).
fixtures:
	./scripts/gen-fixtures.sh

test:
	go test ./engine/... ./narrator/...

build:
	go build ./...
