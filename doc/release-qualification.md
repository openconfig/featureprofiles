# Running Tests for Release Qualification

This guide is for anyone running featureprofiles tests to produce results for
release qualification with the `-jsonl` test result ledger. If you run tests
without `-jsonl`, you do not need anything here.

Release qualification has three parts:

1.  Google shares a `ReleaseIntent` textproto
    ([release_intent.proto](https://github.com/openconfig/qualification-reporter/blob/main/proto/release_intent.proto))
    that lists the tests excluded from the release. Every other test in the
    featureprofiles test registry is expected to run.
2.  You run the featureprofiles tests with `-release_intent` and `-jsonl`.
    [Ondatra](https://github.com/openconfig/ondatra#json-lines-test-report)
    appends one JSON-encoded result per test run to the ledger file.
3.  You turn the ledger into a qualification report with the
    [Qualification Reporter](https://github.com/openconfig/qualification-reporter#readme)
    `generate_report` CLI.

This guide covers part 2: how to run the tests so that every result is
recorded, and which situations leave no record.

## Flags

Flag                     | Defined by       | Purpose
------------------------ | ---------------- | -------
`-release_intent=<path>` | featureprofiles  | Skips tests whose plan ID is listed in `excluded_tests` of the `ReleaseIntent`; all other tests run. Use an absolute path.
`-jsonl=<path>`          | Ondatra          | Appends one result per test run to this JSON-Lines file. Use an absolute path. See [JSON-Lines Test Report](https://github.com/openconfig/ondatra#json-lines-test-report).
`-run_time=<duration>`   | Ondatra          | Time limit for the test run, excluding the wait for the testbed. When it expires, a FAIL result is recorded. See [Running an Ondatra Test](https://github.com/openconfig/ondatra#running-an-ondatra-test).
`-timeout=<duration>`    | `go test`        | Go's own limit per test binary. It defaults to 10m, and when it expires the process stops without recording a result. Set it above `-run_time`.

Use absolute paths for `-release_intent` and `-jsonl`. `go test` runs each test
binary in its own package directory, so a relative path is resolved against
each test's directory: a relative `-release_intent` is not found, and a
relative `-jsonl` scatters results across one file per test directory. Keep
both files outside the featureprofiles checkout; files inside it make the
checkout dirty, so results would be recorded with `gitDirty: true`.

## Example

Run from the root of a clean featureprofiles checkout:

```shell
go test ./feature/... -p 1 -timeout=2h \
  -binding=$PWD/topologies/otgdut_4.binding \
  -release_intent=/abs/path/release_intent.textproto \
  -jsonl=/abs/path/results.jsonl \
  -run_time=110m
```

`-p 1` runs one test package at a time. By default `go test` runs several
packages in parallel, which would make tests compete for the same testbed.

Several runs, including parallel runs on the same machine, can append to the
same ledger file; appends are protected by a file lock. Ledgers from different
machines can be concatenated, for example
`cat rig1.jsonl rig2.jsonl > results.jsonl`.

Each ledger line is a
[`TestResult`](https://github.com/openconfig/ondatra/blob/main/proto/test_result.proto),
for example:

```json
{"testId":"RT-7.2","status":"PASS","executionTimestamp":"2026-11-05T10:00:00Z","gitCommit":"9f1c2ab4d7e0","gitDirty":false}
```

## Test IDs

The `testId` of a result is the test's plan ID, taken from the `plan_id` field
of the `metadata.textproto` file in the test's directory. This is the ID that
`excluded_tests` and the test registry use.

The metadata file is read from the current working directory, which `go test`
sets to the test's package directory. If the file is missing or cannot be read,
or you run a prebuilt test binary from another directory:

*   Without `-release_intent`, the test still runs, but its result is recorded
    under the test binary name (for example `community_test`) instead of the
    plan ID. That ID does not match the test registry, so the test counts as
    missing.
*   With `-release_intent`, the test cannot be matched against the intent, so
    the run stops before the test starts and nothing is recorded.

## What gets recorded

Situation | Ledger result | What to do
--------- | ------------- | ----------
The test runs to completion | PASS or FAIL | None.
The test is listed in `excluded_tests` | None; the test is skipped and prints `--- SKIP` | None. Excluded tests are not part of the qualification.
Ondatra setup fails before any test case starts: invalid Ondatra flag value (for example a missing testbed), binding error or reservation error | FAIL with the error in `statusDetails` | Fix the setup and rerun.
`-run_time` expires | FAIL, "test timed out after ..." | Investigate, then rerun.
`-release_intent` is invalid: the file cannot be read or parsed, an `excluded_tests` entry has an empty `test_id`, or the test has no plan ID | None; the run stops with `Invalid --release_intent: ...` before Ondatra starts | Fix the intent path or file, or the test's `metadata.textproto`, and rerun.
`metadata.textproto` is missing and `-release_intent` is not set | Recorded under the test binary name | Run with `go test` from the checkout so the metadata file is found, and rerun.
Go rejects a command-line flag (for example an unknown flag), or the test does not compile | None | Fix the command or the build and rerun.
`go test -timeout` expires, the test panics, or the process runs out of memory or is killed (SIGKILL) | None; the process stops before it can write a result | Rerun. Recording these needs an external supervisor process.

A test with no result counts as missing, which is treated as not passed. Rerun
it; `generate_report` keeps only the latest result per test, so the rerun
replaces any earlier result.

## Timeouts

To make sure a hung test is recorded as FAIL instead of missing, set Ondatra's
`-run_time` below `go test -timeout`. `-timeout` applies to each test binary
and defaults to 10 minutes, which is shorter than many featureprofiles tests,
so set it explicitly when running for qualification.

## Git commit

`gitCommit` and `gitDirty` are good to have but best effort: they may be
missing from some results, for example when the run fails before the testbed is
reserved or when the checkout's Git state cannot be read. Their absence does
not invalidate a result.

The commit that counts is the one you pass to
`generate_report -featureprofiles_commit`, which is required and recorded in
the qualification report. Run all tests from a clean checkout of that commit.

## Versions

*   Ondatra v0.14.10 or later records setup failures, such as invalid Ondatra
    flag values, as FAIL. Earlier versions write no result for them.
*   Use a `generate_report` release built with the same or a newer Ondatra
    version than your featureprofiles checkout. See
    [Ondatra compatibility](https://github.com/openconfig/qualification-reporter#ondatra-compatibility).
