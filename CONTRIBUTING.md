# Contributing

## Before Editing

1. Read [`AGENTS.md`](AGENTS.md) and the affected package's docs.
2. Run the affected package's deterministic tests as described below.
3. Identify owned dependencies and reverse dependants in `modules.json`.
4. Preserve unrelated work and generated/corpus provenance.

## Changes

The repository contract is provided by the released golib CLI. Use the thin
Makefile targets for local checks; package-specific verification remains in
the repository while shared gate implementations and tool versions are
maintained by go-library-tools.

Keep commits focused and conventional. Update every affected changelog with
the behavior and migration impact. Public API changes require compatibility
evidence and documentation. Specification behavior requires a decision record,
fixture coverage, and interoperability evidence.

New direct dependencies and dependency updates must follow the
[dependency governance policy](AGENTS.md#dependencies-and-supply-chain). Package-local
update bots are forbidden; the root policy owns every module and action update.

Specification-backed changes must follow the
[specification governance contract](AGENTS.md#design), update
the affected stable decision entries, and complete the Specification Decisions
section of the pull request template. An unresolved interpretation or stale
source pin is release-blocking; peer behavior cannot silently select policy.

Selected full/release mutation gates must finish with zero surviving viable
mutants; mutation is not an ordinary local or pull-request prerequisite.

Do not add package-local workflows, permanent replacements, machine-specific
paths, bypass flags, broad mutation exclusions, or aggregate quality metrics
that hide a failing package.

## Verification

Use Go 1.27.2 for verification; the module/API minimum remains Go 1.27.0.
An isolated checkout needs no sibling repositories or coordination goal files.
Begin with deterministic tests and one active target when its byte-adapter
boundary changes, using a disposable cache for each bounded session:

```bash
(
  set -eu
  GOCACHE="$(mktemp -d)"
  export GOCACHE
  trap 'rm -rf "${GOCACHE}"' EXIT
  GOWORK=off go test ./...
  GOWORK=off go test . -run '^$' -fuzz '^FuzzByteAdaptersPreserveCallerBounds$' -fuzztime=2s -timeout=2m
)
```

The deterministic command executes fuzz seeds/corpus, not active fuzzing.
The intentional `benchmarks/comparison` and `integration/resilience` modules
remain separate from the root; when their contracts change, run their tests
from those directories with `GOWORK=off` and no sibling replacements.
Install the verified public v2.1.1 binary archive for your platform from
[go-library-tools releases](https://github.com/faustbrian/go-library-tools/releases/tag/v2.1.1),
checking `checksums.txt` against `.golib.yaml` and the archive against that
checksum set. Put the extracted `golib` binary on `PATH`; do not build Tools
from source. CI verifies the same published release while checking out its
pinned reusable workflow source.

```bash
make inventory
make check
make ci
make specification-check
golib mutation --module . --package .
```

Run adjacent Make commands in one disposable-cache session as shown above.
`make check` and `make ci` run the bounded local contract across the declared
modules, without service or test-tag prerequisites. CI also validates
repository/cohesion structure. Specification checking is offline unless
explicitly requested online. The targeted root race runs once with a two-minute
timeout: 128 contenders must receive exactly 32 injections, and concurrent
selection/reset/snapshot access must retain the single-rule/single-fault bound.
Base-free checks run seeds/corpus; changed-source CI selects causal active
fuzz smoke. Optional mutation is limited to one package, one worker and a
twenty-minute deadline.

Select full qualification and release rehearsal explicitly:

```bash
make check-full
make specification-check-online
golib api check
golib release dry-run --module .
```

Full checks retain all four declared fuzz targets at 10000x/20m each, the
one-second benchmark operation, intentional integration modules and all
original module gates. The scheduled carrier uses the shared bounded route;
scheduling does not itself select full qualification. Online specification
checking fetches authority/errata sources when applicable. Release rehearsal
can run locally. Report unavailable or failing commands; bounded checks alone
do not establish release qualification.

## Selected Monthly Assurance

[`assurance.yml`](.github/workflows/assurance.yml) runs on main at 07:11 UTC on
the 7th day of each month. It selects one of the 4 declared root-module fuzz
targets using the shared UTC-week selector; it does not run every target or
promise a complete monthly rotation. Explicit main-only dispatch accepts one
exact declared selector, for example `^FuzzByteAdaptersPreserveCallerBounds$`.

Configuration acceptance and bounded reader/writer adapters are the selected
risks: the existing targets check bounded decisions, returned byte counts and
caller-input preservation. This is not composed resilience or concurrent-budget
qualification; retain the targeted race checks and affected nested-module tests.

The immutable shared runner bounds active fuzzing to 30 seconds, two workers
and two CPUs, with a five-minute combined native session, 2 GiB memory and
1 MiB output limits inside a ten-minute job. It installs the declared public
CLI without a source build, uses private disposable caches and keeps failures,
timeouts and exhausted budgets visible. Ordinary deterministic tests still run
seeds/corpus; ordinary CI and its Required contract are unchanged. Full/manual
operations and their original budgets remain in `.golib.yaml`.

Treat findings as owner-attributed diagnostics: promptly repair viable defects
with focused regressions; retain reviewed narrow equivalent/invalid dispositions
rather than weakening tests. Review cadence against useful findings and actual
cost; do not interpret a selected pass as full or release qualification.

## Adding A Module

Follow [repository structure policy](AGENTS.md#repository-structure). New modules
require an explicit purpose, ownership boundary, dependency review, package
catalog entry, full quality gates, documentation, changelog, license, security
policy, compatibility plan, and release dry-run.
