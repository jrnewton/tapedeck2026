# Summary
You are building a simple web application that is used to download radio station archive streams (typically stored in mp3 files).

# WORKLOG
WORKLOG.md contains all work items one per line in the following format:
<STATUS>: <ITEM>, where STATUS is TODO, PROG, DONE and ITEM is a short description of the work.

Indented under each work item can be more details as needed.

Eg
DONE: create DESIGN.MD
  determine languages to use
  deployment architecture
PROG: implement frontend
TODO: implement backend

# Design
See DESIGN.md for living document of the project's technical design.
The target OS and architecture is a docker container running under a Linux host system.

# Git
You are working in a Git repository.  Pls commit after each iteration is done and tested.

# Go Code
- Project structure must be as simple as possible.
- Always ask when introducing third party code.

# Makefile
Use `make` for common tasks:
- `make build` - Build binaries
- `make run` - Run server via Docker Compose (port 8080)
- `make stop` - Stop server
- `make logs` - View server logs
- `make test` - Run unit tests
- `make test-e2e` - Run E2E tests (requires Docker with Chromium)
- `make clean` - Remove build artifacts
