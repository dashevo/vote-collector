# System setup
SHELL = bash

COVERAGE_FILE ?= coverage.txt

#.PHONY: default
default:  goimports lint vet build #test ## Run default target : all lints + test

build:  ## Build the package
	go build

cicheck:  ## Run basic code checks
	diff -u <(echo -n) <(goimports -d .)
	diff -u <(echo -n) <(golint ./...)
	go vet ./...

test:  ## Run a basic test suite
	go test
	#go test -v

cover:  ## Run tests and generate test coverage file, output coverage results and HTML coverage file.
	go test -coverprofile $(COVERAGE_FILE)
	go tool cover -func=$(COVERAGE_FILE)
	# go tool cover -html=$(COVERAGE_FILE)
	rm -f $(COVERAGE_FILE)

goimports:  ## Run goimports to format code
	goimports -w .

lint:  ## Lint all go code in project
	golint ./...

vet:  ## Go vet all project code
	go vet ./...

help:  ## Show This Help
	@for line in $$(cat Makefile | grep "##" | grep -v "grep" | sed  "s/:.*##/:/g" | sed "s/\ /!/g"); do verb=$$(echo $$line | cut -d ":" -f 1); desc=$$(echo $$line | cut -d ":" -f 2 | sed "s/!/\ /g"); printf "%-30s--%s\n" "$$verb" "$$desc"; done

clean:  ## Clean up transient (generated) files
	go clean
	rm -f $(COVERAGE_FILE)
