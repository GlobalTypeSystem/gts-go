# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project Overview

`gts-go` is the Go reference implementation of [GTS](https://github.com/GlobalTypeSystem/gts-spec) — library (`gts/`), CLI (`cmd/gts`), and an HTTP server (`cmd/gts-server` / `gts server`) that answers the REST API exercised by the shared gts-spec conformance suite.

`.gts-spec/` is the spec vendored as a git submodule; `tests/` inside it is the conformance suite. See `README.md` for API/CLI details and `make help` for all targets.

## Running the gts-spec Test Suite

The short form is `make e2e PORT=8001` (PORT defaults to 8000; override if busy — the target fails fast if the port is already in use). It bootstraps the venv, rebuilds, starts the server, runs pytest, and shuts everything down.

### Bootstrap the venv (first time only)

Tests depend on `httprunner`. Installed into `.gts-spec/.venv/` (gitignored by the submodule).

```bash
make e2e-venv                     # uses python3 by default
make e2e-venv PYTHON=python3.11   # Python 3.11 is the safest (httprunner still pins pydantic<2)
```

### Run pytest manually against a running server

Useful when iterating on a single test without the full `make e2e` cycle.

```bash
make build
./bin/gts server --port 8001 &

PYTEST=".gts-spec/.venv/bin/python -m pytest"

# Whole suite
$PYTEST .gts-spec/tests --gts-base-url http://127.0.0.1:8001

# One file / one class
$PYTEST .gts-spec/tests/test_refimpl_x_gts_final_abstract.py --gts-base-url http://127.0.0.1:8001
$PYTEST .gts-spec/tests/test_op12_schema_vs_schema_validation.py::TestCaseOp12_FinalBase_RejectDerived --gts-base-url http://127.0.0.1:8001
```

`GTS_BASE_URL` env var works too. The server holds state in memory with no reset endpoint — restart between full-suite runs.

### Open-files limit on macOS

Before running the full suite, raise the FD limit in the calling shell:

```bash
ulimit -n 4096
```

`httprunner` leaks a keep-alive socket per test class (the underlying
`requests.Session` is never closed), so FDs grow linearly. macOS's
default 256 soft cap is hit around test ~240 with `EMFILE: Too many
open files`. Linux defaults (1024+) are usually fine. `make e2e` runs
its commands in a subshell, so the `ulimit` must be set in the parent
shell that invokes `make`. Don't bake `ulimit` into the Makefile —
it's an environment concern, not a build step.

## Working in This Repo

- `.gts-spec` is a submodule. Bump with `make update-spec`, then rerun `make e2e` to pick up new spec tests.
- Handlers in `server/` stay thin — logic goes in `gts/` where it is unit-testable. New REST behavior usually already has coverage in `.gts-spec/tests/`; run the relevant file before and after to confirm.
- `make check` is the full local gate: fmt + vet + lint + test + e2e.
