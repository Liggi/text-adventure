#!/usr/bin/make -f

.DEFAULT_GOAL := run

run:
	@./scripts/run.sh

dev:
	@./scripts/dev.sh

debug: reset cleanlogs
	@./scripts/debug.sh

build:
	@./scripts/build.sh

clean:
	@./scripts/clean.sh

worldstate:
	@./scripts/worldstate.sh

reset:
	@./scripts/reset.sh

cleanlogs:
	@echo "Clearing debug logs..."
	@rm -f debug.log
	@echo "Debug logs cleared."

.PHONY: run dev debug build clean worldstate reset cleanlogs