# The `addrundata` Tool

The `addrundata` tool keeps all the `metadata.textproto` files up to date in the
featureprofiles repo. These files contain rundata that identify each test,
and the rundata will find its way into the test XML output when a functional
test is run with the `-xml` flag. The rundata allow us to track the test result.

There are two modes of operation:

*   Check mode: `go run ./tools/addrundata`

    When run without a flag, this checks the integrity of rundata in the repo.
    This is used by the "[Rundata Check]" pull request check. If the check
    fails, we will not be able to track the test result from those tests with
    outdated rundata.

*   Fix mode: `go run ./tools/addrundata --fix`

    This will update any outdated rundata, to be run by the author of a pull
    request if the "[Rundata Check]" fails.

[Rundata Check]: /.github/workflows/rundata_check.yml

An example `metadata.textproto` looks like this:

```
# proto-file: proto/metadata.proto
# proto-message: Metadata

uuid: "bf60afdc-7130-4bef-a23c-39783c7f2bb3"
plan_id: "XX-1.1"
description: "Foo Functional Test"
```

Both `plan_id` and `description` are sourced from the top-level heading in
`README.md`:

```md
# XX-1.1: Foo Functional Test

## Summary

One line summary of what foo functional test does.
```

But the `uuid` is uniquely generated for each test. The `addrundata` tool takes
care of the UUID generation. Both the `ate_tests` and `otg_tests` variants of
the same test must have the same rundata.
