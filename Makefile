#!/usr/bin/make -f

.DEFAULT_GOAL := run

run:
	@go run main.go completions.go

review:
	@go run main.go completions.go review

rate:
	@echo "Usage: make rate ID=123 RATING=4 NOTES=\"good response\""
	@if [ -z "$(ID)" ] || [ -z "$(RATING)" ]; then exit 1; fi
	@go run main.go completions.go rate $(ID) $(RATING) $(NOTES)

build:
	@go build -o text-adventure main.go completions.go

clean:
	@rm -f text-adventure completions.db debug.log

.PHONY: run review rate build clean