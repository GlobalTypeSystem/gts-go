# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project Overview

`gts-go` is the Go reference implementation of [GTS](https://github.com/GlobalTypeSystem/gts-spec) — library (`gts/`), CLI (`cmd/gts`), and an HTTP server (`cmd/gts-server` / `gts server`) that answers the REST API exercised by the shared gts-spec conformance suite.

The spec version this implementation targets is pinned in
[`.gts-spec-version`](.gts-spec-version) (used verbatim as the GHCR tag for
the test-runner image). Bumping the pin and running the suite is how new
spec features land. See `README.md` for API/CLI details and `make help` for
all targets.

## Running the gts-spec Test Suite

`make gts-spec-tests` runs the gts-spec conformance suite against a freshly
built server. Tests come from the published runner image
`ghcr.io/globaltypesystem/gts-spec-tests`; the tag comes from
[`.gts-spec-version`](.gts-spec-version) as an immutable
`vMAJOR.MINOR.PATCH` — every commit reproduces the same test run, and
rolling forward is a deliberate bump of that file. Requires a working
Docker daemon plus the Go toolchain (the target builds the server binary
before pulling the test-runner image).

```bash
make gts-spec-tests                                    # full suite on :8000
make gts-spec-tests PORT=8001                          # different port
make gts-spec-tests TEST=test_op1_id_validation.py     # single file / selector
```

Opt into the rolling minor tag, try a different patch, or test a fork:

```bash
make gts-spec-tests GTS_SPEC_VERSION=v0.11             # rolling vMAJOR.MINOR
make gts-spec-tests GTS_SPEC_VERSION=v0.11.0           # specific patch
make gts-spec-tests GTS_SPEC_IMAGE=ghcr.io/your-fork/gts-spec-tests
```

Iterating on the test suite itself? Mount a local checkout over `/tests`:

```bash
make gts-spec-tests GTS_SPEC_TESTS_DIR=../gts-spec/tests
```

For tight test-edit loops, keep a long-running server in one terminal and
re-run targeted tests in another:

```bash
# Terminal 1
make gts-server PORT=8001

# Terminal 2
make gts-spec-tests-run PORT=8001 TEST=test_op12_type_derivation_validation.py
```

## Working in This Repo

- The spec is no longer a git submodule. To bump the conformance target,
  edit `.gts-spec-version` and run `make gts-spec-tests` — the new image
  is pulled on demand.
- Handlers in `server/` stay thin — logic goes in `gts/` where it is
  unit-testable. New REST behavior usually already has coverage in the
  gts-spec test suite; iterating on a single test via
  `make gts-spec-tests TEST=...` is the fastest signal.
- `make check` is the full local gate: fmt + vet + lint + test +
  gts-spec-tests.
