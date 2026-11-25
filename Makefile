VERSION ?= dev
LDFLAGS := -X github.com/propedeutica/svmm-agent/internal/agent.Version=$(VERSION)

.PHONY: build test lint
build:
	go build -ldflags "$(LDFLAGS)" -o bin/svmm-agent ./cmd/svmm-agent

test:
	go test -race -cover ./...

lint:
	gofmt -l . && go vet ./...
