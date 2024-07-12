# Contributing the Openconfig Feature Profiles

Thank you for your interest in contributing to OpenConfig feature profiles.

## Rationale

See the [README](README.md) for an explanation of what OpenConfig feature
profiles are and why we have them.

## Ways to Contribute

OpenConfig prefers contributions in the form of code, documentation and even bug
reporting. If you wish to discuss the suitability or approach for a change this
can be done with an issue in the
[OpenConfig feature profiles GitHub](https://github.com/openconfig/featureprofiles/issues).

## Contributor License Agreement

All contributions to OpenConfig feature profiles MUST be Apache 2.0 licensed.
The
[Google contributor license agreement (CLA)](https://cla.developers.google.com/),
MUST be signed for any contribution to be merged into the repository.

The CLA is used to ensure that the rights to use the contribution are well
understood by the OpenConfig community. Since copyright over each contribution
is assigned to its authors, code comments should reflect the contribution made,
and the copyright holder. No code will be reviewed if the license is not
explicitly stated, or the CLA has not been signed.

Note that we use the Google CLA because the OpenConfig project is
[administered and maintained by Google](https://opensource.google.com/docs/cla/#why),
not to ascribe any specific rights to a single OpenConfig contributor.

## Code Style

All code should follow
[Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments),
[Effective Go](https://go.dev/doc/effective_go), and
[Google Go Style Guide](https://google.github.io/styleguide/go/) for writing
readable Go code with a consistent look and feel.

All code and documentation should follow
[Google developer documentation style guide](https://developers.google.com/style/word-list)
for the use of inclusive language.

It is recommended that tests follow
[Testing on the Toilet](https://testing.googleblog.com/) for best practices on
design patterns.

Here is a specific list of test design patterns that we follow:

*   [Table Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests),
    except we do not want test cases to run in parallel.
*   [Testing on the Toilet: Don't Put Logic in Tests](https://testing.googleblog.com/2014/07/testing-on-toilet-dont-put-logic-in.html).
*   [Code Health: Eliminate YAGNI Smells](https://testing.googleblog.com/2017/08/code-health-eliminate-yagni-smells.html).
*   [Why does Go not have assertions?](https://go.dev/doc/faq#assertions)

## Directory Organization

The directory tree is organized as follows:

*   `cloudbuild/` contains google cloud build scripts for running virtual
    routers in containers on [KNE](https://github.com/openconfig/kne)
*   `feature/` contains definition and tests of feature profiles.
*   `feature/experimental` contains tests which have automation which is
    not confirmed to pass on any hardware platform or software release.
    When the test automation is passing against at least one DUT,
    it is moved to the `feature/` directory.
*   `internal/` contains packages used by feature profile tests.
*   `proto/` contains protobuf files for feature profiles.
*   `tools/` contains code used for CI checks.
*   `topologies/` contains the testbed topology definitions.

Directory names are not allowed to contain hyphen (-) characters.

## Allowed File Types

Regular files should be plain text in either ASCII or UTF-8 encoding. Please
omit empty files.

Regular files should not have the executable bit. In particular:

*   Source code should not be executable. That's because they have to be
    compiled into a binary before they can be executed.
*   Text-formatted protos are not executable. These including the testbed and
    binding files.
*   Device configs are not executable.

Exceptions are the scripts (Shell, Python or Perl) for generating code or for
checking the integrity of the repository. Scripts should have the executable bit
set.

Please do not check in binary files. If there is a need to expand the scope of
allowed file types, please file an issue for discussion.

## Test Suite Organization

Test suites should be placed in subdirectories formatted like
`feature/<featurename>/[<sub-feature>/]<tests|otg_tests|kne_tests>/<test_name>/<test_name>.go`.
For example:

*   `feature/interface/` is the collection of interface feature profiles.
*   `feature/interface/singleton/` contains the singleton interface feature
    profile.
*   `feature/interface/singleton/README.md` - documents the singleton interface
    feature profile.
*   `feature/interface/singleton/feature.textproto` - defines the singleton
    interface feature profile in machine readable format.
*   `feature/interface/singleton/ate_tests/` contains the singleton interfaces
    test suite using ATE traffic generation API.  Note, use of the ATE API is
    deprecated and should not be used for any new test development.
*   `feature/interface/singleton/otg_tests/` contains the singleton interfaces
    test suite using OTG traffic generation API.
*   `feature/interface/singleton/kne_tests/` contains the singleton interfaces
    test suite that are intended to only run under KNE and not on hardware
    devices.
*   `feature/interface/singleton/tests/` contains the singleton interfaces test
    suite without traffic generation.
*   `internal/deviations` contains code which overrides test behavior where
    there are known issues in a DUT. Follow the guidelines posted at
    `internal/deviations/README.md` to add new deviations.

Within each test directory, `README.md` should document the test plan. The test
name directory and the `*.go` files should be named after the test name as shown
in the [project](https://github.com/orgs/openconfig/projects/2/views/1) item.

Each test must also be accompanied by a `metadata.textproto` file that supplies
the metadata for annotating the JUnit XML test results. This file can be
generated or updated using the command: `go run ./tools/addrundata --fix`. See
[addrundata](/tools/addrundata/README.md) for more info.

For example:

*   `feature/interface/singleton/otg_tests/singleton_test/README.md` - documents
    the test plan for the issue
    [RT-5.1 Singleton Interface](https://github.com/openconfig/featureprofiles/issues/111).
*   `feature/interface/singleton/otg_tests/singleton_test/singleton_test.go`
    implements the issue.
*   `feature/interface/singleton/otg_tests/singleton_test/rundata_test.go`
    contains the rundata.

## Code Should Follow The Test Plan

The test plan in `README.md` is generally structured like this:

```
# RT-5.1: Singleton Interface

## Summary

...

## Procedure

1. Step 1
2. Step 2
3. ...

## Config Parameter Coverage

*   /interfaces/interface/config/name
*   /interfaces/interface/config/description
*   ...

## Telemetry Parameter Coverage

*   /interfaces/interface/state/oper-status
*   /interfaces/interface/state/admin-status
*   ...
```

Each step in the test plan procedure should correspond to a comment or `t.Log`
in the code. Steps not covered by code should have a TODO.

In the PR, please mention any corrections made to the test plan for errors that
were discovered when implementing the code.

## Test Structure

Generally, a Feature Profiles ONDATRA test has the following stages: configure
DUT, configure OTG, generate and verify traffic, verify telemetry. The
configuration stages should be factored out to their own functions, and any
subtests should be run under `t.Run` so the test output clearly reflects which
parts of the test passed and which parts failed.

They typically just report the error using `t.Error()` for checks. This way, the
error message is accurately attributed to the line of code where the error
occurred.

```
func TestFoo(t *testing.T) {
  configureDUT(t) // calls t.Fatal() on error.
  configureOTG(t) // calls t.Fatal() on error.
  t.Run("Traffic", func(t *testing.T) { ... })
  t.Run("Telemetry", func(t *testing.T) { ... })
}
```

In the above example, `configureDUT` and `configureOTG` should not be subtests,
otherwise they could be skipped when someone specifies a test filter. The
"Traffic" and "Telemetry" subtests will both run even if there is a fatal
condition during `t.Run()`.

### Table Driven Tests

Each case in a table driven test should also be delineated with `t.Run()` as a
subtest and should have a symbolic name and a description. The description text
should be a direct quote from the test plan. The symbolic name allows test
filtering, and the description should be logged at the beginning of the subtest.

```
func TestTableDriven(t *testing.T) {
  cases := []struct{
    name, desc string
    ...
  }{
    ...
  }
  for _, c := range cases {
    t.Run(c.name, func(t *testing.T) {
      t.Log("Description: ", c.desc)
      configureDUT(t, /* parameterized by c */)
      configureOTG(t, /* parameterized by c */)
      t.Run("Traffic", func(t *testing.T) { ... })
      t.Run("Telemetry", func(t *testing.T) { ... })
    })
  }
}
```

If the table driven test does not change either the DUT or the ATE between
cases, these stages may be moved out of the for-loop.

### Subtests vs. Test Helpers

When the setup is more involved, it is often necessary to break test code into
separate functions as subtests, or rely on test helpers. The way we distinguish
between subtests and test helpers is by their arguments.

#### When to Use a Subtest

A subtest or a portion of a test takes `t *testing.T` as the argument. A portion
of a test implements what is explicitly described in the test plan procedure,
typically limited to a single step.

```
// configureIPv4ViaClientA configures an IPv4 entry via client A with Election ID of 12.
// Ensure that the entry is installed.
func configureIPv4ViaClientA(t *testing.T, client *fluent.GRIBIClient) {
  // Do not call t.Helper()
}
```

Generally, most code in `foo_test.go` should be test code.

#### When to Use a Test Helper

On the other hand, a test helper provides an implementation detail below what is
specified in the test plan. They are often reusable across many tests.
Generally, test helpers should be a package under `internal`, e.g.
`internal/gribi`.

It is recommended that a test helper simply [returns error as usual][errors] and
does not report test errors on its own. When necessary, it may accept `t
testing.TB` as an argument if it has to report `t.Error()`, in which case it
must call `t.Helper()` as the first statement, so the test error is attributed
to the caller instead of to the helper.

[errors]: https://github.com/golang/go/wiki/CodeReviewComments#error-strings

```
// fooHelper is recommended.
func fooHelper(...) error {
  ...
}

// barHelper is also OK.
func barHelper(t testing.TB, ...) {
  t.Helper()
  // Any t.Error() in the code is attributed to the caller.
}
```

Don't do both. If a helper returns an error value and still reports `t.Error()`,
it creates redundant and possibly divergent error paths that the caller will
have to remember checking.

```
// bazHelper is NOT ok because it mixes error reporting.
func bazHelper(t testing.TB, ...) error {
  t.Error(...)  // Don't do this.
}
```

Do not write [assertion] helpers.

[assertion]: https://go.dev/doc/faq#assertions

## Enum

Sometimes a test may need to set a ygot field with an OpenConfig enum type, e.g.
[IP_PROTOCOL]. The constant for `IP_TCP` has the description "Transmission
Control Protocol (6)". The value `6` here refers to the IANA-assigned
[protocol numbers], but ygot-generated code assigns enum values sequentially,
and `PacketMatchTypes_IP_PROTOCOL_IP_TCP` in [enum.go] actually has the value
`9`.

[IP_PROTOCOL]: http://ops.openconfig.net/branches/models/master/docs/openconfig-acl.html#ident-ip_protocol
[protocol numbers]: https://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml
[enum.go]: https://github.com/openconfig/ondatra/blob/main/telemetry/enum.go

This is okay:

```go
acl := d.GetOrCreateAcl().GetOrCreateAclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4)
a1 := acl.GetOrCreateAclEntry(1)
a1v4 := a1.GetOrCreateIpv4()
a1v4.Protocol = telemetry.PacketMatchTypes_IP_PROTOCOL_IP_TCP
```

This is also okay because the port number is not a numerical enum constant.

```go
const bgpPort = 179
a1t := a1.GetOrCreateTransport()
a1t.DestinationPort = telemetry.UnionUint16(bgpPort)
```

The ygot-generated numerical enum values are internal. When the ygot `GoStruct`
is serialized to JSON, it will output the string `"IP_TCP"`. Do not use the
numerical enum values.

```go
// This is NOT ok because it uses an internal numerical constant.
// Also, the constant actually refers to IP_L2TP, not IP_TCP.
alv4.Protocol = telemetry.UnionUint8(6)

// This is also NOT ok.
a1v4.Protocol, _ = a1v4.To_Acl_AclSet_AclEntry_Ipv4_Protocol_Union(6)
```

## IP Addresses Assignment

> **Warning:** Though we are trying to use RFC defined non-globally routable
> space in tests, there might be tests (e.g. scaling tests) that are still using
> public routable ranges. Users who run the tests own the responsibility of not
> leaking test traffic to internet.

### IPv4

*   192.0.2.0/24 ([TEST-NET-1](https://www.iana.org/go/rfc5737)): control plane
    addresses split into /30 subnets for each ATE/DUT port pair.
*   198.51.100.0/24 ([TEST-NET-2](https://www.iana.org/go/rfc5737)): data plane
    source network addresses used for traffic testing; split as needed.
*   203.0.113.0/24 ([TEST-NET-3](https://www.iana.org/go/rfc5737)): data plane
    destination network addresses used for traffic testing; split as needed.
*   100.64.0.0/10 ([CGN Shared Space](https://www.iana.org/go/rfc6598)):
    additional network address; split as needed.
*   198.18.0.0/15 ([Device Benchmark Testing](https://www.iana.org/go/rfc2544)):
    additional network address; split as needed.

### IPv6

2001:DB8::/32
([Reserved for Documentation](https://datatracker.ietf.org/doc/html/rfc3849)) is
a very large space, so we divide them as follows.

*   2001:DB8:0::/64: control plane addresses split into /126 subnets for each
    ATE/DUT port pair.
*   2001:DB8:1::/64: data plane addresses used for traffic testing as the source
    address; split as needed.
*   2001:DB8:2::/64: data plane addresses used for traffic testing as the
    destination address; split as needed.

Link local addresses (FE80::/10) addresses are allowed in contexts where link
local is being tested.

### Rationale

The properties being tested in the test plan are agnostic to the IP addresses
being used, so tests do not require a specific hard-coded IP address. However,
tests must avoid choosing addresses already used by a public network or a local
network, in order to avoid misconfigured DUT from flooding the network with test
traffic and causing service disruption.

Here are some examples why certain addresses commonly found in networking
tutorials online could be problematic. **DO NOT USE** these addresses.

*   [1.1.1.1](https://bgp.he.net/ip/1.1.1.1) belongs to APNIC and Cloudflare DNS
    Resolver project.
*   [2.2.2.2](https://bgp.he.net/ip/2.2.2.2) belongs to Orange S.A. in France.
*   [9.9.9.9](https://bgp.he.net/ip/9.9.9.9) belongs to Quad9 in Switzerland.
*   [111.111.111.111](https://bgp.he.net/ip/111.111.111.111) belongs to KDDI
    CORPORATION in Japan.
*   [222.222.222.222](https://bgp.he.net/ip/222.222.222.222) belongs to CHINANET
    hebei province network in China.

We also avoid using the private addresses commonly used in a local network, such
as 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, because test traffic destined to
these addresses may disrupt your local network. We also avoid 100.64.0.0/10
which is used by Carrier Grade NAT.

## ASN Assignment

Autonomous System numbers used in test should follow Autonomous System (AS)
Number Reservation for Documentation Use ([RFC 5398]). In particular:

[RFC 5398]: https://datatracker.ietf.org/doc/html/rfc5398

*   16-bit ASN: 64496 - 64511 (`0xfbf0` - `0xfbff`)
*   32-bit ASN: 65536 - 65551 (`0x10000` - `0x1000f`)

Both ranges have 16 total numbers each. The hexadecimal notation makes it more
obvious where the range starts and stops.

## Default Network Instance

In OpenConfig [PR #599](https://github.com/openconfig/public/pull/599), it has
been clarified that the name for the default network instance should be
uppercase `"DEFAULT"`. Some legacy devices are still using lowercase
`"default"`, so device tests should use the deviation
`*deviations.DefaultNetworkInstance` which allows them to work on those legacy
devices while they are being updated. Non-device unit tests may hard-code
`"DEFAULT"`.

## Pull Requests

To contribute a pull request:

1.  Make a fork of the
    [openconfig/featureprofiles](https://github.com/openconfig/featureprofiles)
    repo and make the desired changes, following the
    [GitHub Quickstart](https://docs.github.com/en/get-started/quickstart)
    guide.

1.  When opening a pull request, use a descriptive title and detail. See
    [Pull Request Title](#pull-request-title) below.

    *   Pull requests should be kept small, ideally under 500 lines. Small
        changes allow detailed discussions of the additions that are being made
        to the model, whilst also ensuring that course-corrections can be made
        early in the process. If a PR is growing to more than 500 lines, it may
        need to be broken into multiple smaller PRs.

1.  The pull request should include both the suggested feature profile textproto
    additions, as well as any relevant additions to tests. Tests should be
    written in Go using the [ONDATRA](https://github.com/openconfig/ondatra)
    framework.

1.  We are in the process of migrating tests using the Ondatra ATE API to the
    OTG API, so many tests have both versions. When making changes to an ATE
    test, please port the changes to its OTG test unless it is missing. ATE
    tests are found under the path `ate_tests`, and OTG tests under `otg_tests`.

1.  The automated CI running against each pull request will check the pull
    request for compliance. The author should resolve any issues found by CI.

1.  One or more peers in the community may review the pull request.

1.  A feature profile repository maintainer will be reponsible for a final
    review and approval. Only a feature repository maintainer can merge a pull
    request to the main branch.

The aim of this process is maintain the model quality and approach that
OpenConfig working group has strived for since its inception in 2014. Thank you
for your contributions!

### Pull Request Title

Pull request title should start with the scope (i.e. section in the test plan).

```
RT-5.1 singleton configuration and telemetry, no traffic test yet
```

The description may add more details as desired to benefit the reviewers. The
preferred format is:

```
    * (M) internal/fptest/*
      - Add a helper for referencing a keychain from other modules.
    * (M) feature/isis/otg_tests/base_adjacencies_test
      - Fix testing of hello-authentication to reference a specific
        keychain.
      - Fix authentication of *SNP packets, referencing a keychain
        that can be used to auth these packets.
```
