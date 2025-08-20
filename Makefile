#!/usr/bin/make -f

.DEFAULT_GOAL := run

run:
	@./scripts/run.sh

dev:
	@./scripts/dev.sh

debug:
	@./scripts/debug.sh

build:
	@./scripts/build.sh

clean:
	@./scripts/clean.sh

worldstate:
	@./scripts/worldstate.sh

.PHONY: run dev debug build clean worldstate