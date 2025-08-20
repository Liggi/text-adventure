#!/usr/bin/make -f

.DEFAULT_GOAL := run

run:
	@./scripts/run.sh

dev:
	@./scripts/dev.sh

build:
	@./scripts/build.sh

clean:
	@./scripts/clean.sh

worldstate:
	@./scripts/worldstate.sh

.PHONY: run dev build clean worldstate