.PHONY: test build clean format vet lint race bench guard all

test:
	go test ./...

build:
	go build ./...

clean:
	go clean ./...

format:
	gofmt -l -w .

vet:
	go vet ./...

lint:
	golangci-lint run ./...

race:
	go test -race ./...

bench:
	go test -bench=. -benchmem ./...

# guard runs the checks that must pass before a commit.
guard: format vet race
	@echo "All guards passed."

all: guard build
