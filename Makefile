CONSENSUS_IMAGE=blockemulator/consensusnode:latest
SUPERVISOR_IMAGE=blockemulator/supervisor:latest

.PHONY: all
all: test lint-fix build-image

.PHONY: test
test:
	go test -gcflags=all='-N -l' ./...

.PHONY: lint-fix
lint-fix:
	golangci-lint run ./... --fix

.PHONY: build-image
build-image:
	docker build -t $(CONSENSUS_IMAGE) --target consensusnode .
	docker build -t $(SUPERVISOR_IMAGE) --target supervisor .

.PHONY: run-consensus
run-consensus:
	docker run --rm $(CONSENSUS_IMAGE) $(ARGS)

.PHONY: run-supervisor
run-supervisor:
	docker run --rm $(SUPERVISOR_IMAGE) $(ARGS)

.PHONY: docs-pdf2svg
docs-pdf2svg:
	sh ./docs/scripts/pdf2svg.sh