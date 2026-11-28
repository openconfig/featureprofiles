# AFT global-filter tests with typed Ondatra bindings

The AFT-6.1 through AFT-6.4 tests use generated paths beneath
`gnmi.OC().NetworkInstance(name).Afts().GlobalFilter()` for policy updates,
deletion, expected-error checks, and CONFIG/STATE readback. Dual-stack changes
remain batched into one Set RPC, and Cisco uses JSON_IETF string encoding.
Readback explicitly uses gNMI Get, including the fresh transport after reboot.
Resilience readback uses four typed leaf Get requests in place of two paired
requests.

Direct subscriptions remain where a test must inspect notification deletion
or atomic flags. Their global-filter paths come from generated state queries.
These protocol assertions are separate from manual policy configuration paths.

## Dependency

Upstream Ondatra v0.15.1 uses OpenConfig models v5.11.0 but omits
`openconfig-aft-global-filter.yang` from its generator inputs. The pinned public
B4FireX Ondatra fork adds this input and regenerates the OC bindings from that
release. It is a regeneration of the public release, rather than an unchanged
upstream release. See [openconfig/ondatra#159](https://github.com/openconfig/ondatra/issues/159).

The replacement module in `go.mod` is public and immutable. No private Git
credentials or sibling checkout are required. Go 1.26.3 is required by Ondatra.

The checkout selects the dependency automatically:

```go
require github.com/openconfig/ondatra v0.15.1

replace github.com/openconfig/ondatra => github.com/b4firex/ondatra v0.0.0-20261120174419-ff486025d4fa
```

`gnmi/generate.sh` is a source-generation script in the Ondatra repository.
Featureprofiles consumes the generated Go packages through this module pin;
it does not execute that script while building or running a test. The added
generator input is `public/release/models/aft/openconfig-aft-global-filter.yang`.
Updating only the model version at line 21 does not add that input.

## Jenkins / FireX

The bindings update is part of the checkout through `go.mod` and `go.sum`.
The build must preserve that replacement and use Go 1.26.3 or later; no manual
binding regeneration or private Ondatra checkout is needed.

Select the upstream PR in `openconfig/featureprofiles`. Its source branch is
hosted in the B4FireX fork. In Jenkins, choose `Public` and enter the upstream
number in `Test PR`. For an existing B4 runlist, the equivalent source settings
are:

```yaml
internal_test: false
test_repo_url: https://github.com/openconfig/featureprofiles.git
```

Select the upstream PR's number with the job's Test PR setting. Clear the old
internal `test_branch`, any explicit test revision, and any repository override
that points to the fork. Leave the job's Test branch and Test commit hash fields
empty. Keep the existing AFT package selections and intended testbed/image
profile.

Use the established B4 SIM runtime profile described below when reproducing
the local simulator results. The module pin supplies the bindings; the normal
runner still supplies the simulator, binding/testbed files, service access and
traffic-generator setup.

## Running a checkout

Use Go 1.26.3 or later, a C toolchain and libpcap development headers. The
normal binding and testbed files must describe the selected DUT and ATE. Go
fetches the pinned public dependency; do not edit module replacements locally
or copy generated bindings into the checkout.

The following checks do not reserve or configure a testbed:

```bash
go list -mod=readonly -m github.com/openconfig/ondatra
go test -mod=readonly ./internal/aft_binding_contract
```

For example, with absolute paths to a prepared binding, testbed and output
directory, AFT-6.3 can be selected with:

```bash
go test -mod=readonly -v -count=1 -timeout=60m \
  ./feature/afts/filtered_streaming/otg_tests/afts_prefix_filtering_resilience \
  -args -binding="$AFT_BINDING" -testbed="$AFT_TESTBED" \
  -outputs_dir="$AFT_OUTPUTS"
```

That command executes a device reboot. Run each suite on an appropriate owned
testbed. The package selections and overall deadlines used for validation are:

| Suite | Directory under `feature/afts/filtered_streaming/otg_tests/` | Go timeout |
|---|---|---|
| AFT-6.1 | `afts_prefix_filtering` | `90m` |
| AFT-6.2 | `afts_prefix_filtering_dualstack` | `30m` |
| AFT-6.3 | `afts_prefix_filtering_resilience` | `60m` |
| AFT-6.4 | `afts_prefix_filtering_dynamic` | `60m` |

### Matching the simulator results

The passing local runs built a separate runtime copy with the B4 SIM overlay
`internal/fptest/otg_patch.go` and its gomonkey v2.14.0 dependency. That overlay
adds a ten-second delay before traffic and doubles `context.WithTimeout`
deadlines process-wide. It is not included in this featureprofiles change.
A plain `go test` invocation above therefore does not reproduce that entire
runtime profile by itself. Matching the prior SIM results requires the same
runner-provided overlay, image and OTG version, with a fresh SIM for each suite.
The SIM runs also supplied
`-deviation_ate_port_link_state_operations_unsupported`.

Before declaring the SIM ready, enable and commit both test interfaces, verify
native Up/Up state after OTG setup, and repeat that check immediately before
test launch. BGP establishment and expected route counts must also succeed.
The previously observed startup failures occurred before AFT feature checks;
the bindings pin cannot repair interface or BGP readiness.

The dependency is reproducible from the checkout. AFT-6.1 was rerun from the
prepared public checkout with this profile. The table below distinguishes
that run, the existing AFT-6.2/6.3 evidence, and the AFT-6.4 runtime gap.

## AFT-6.3 hardware reboot readiness

AFT-6.3 waits separately for serving gNMI telemetry with an increased boot
time, then for BGP sessions and the expected route populations to recover.
Its post-reboot checks use a refreshed test-owned transport, including the
BGP session, prefix-count and VRF AFT-summary reads.

| Flag | Default | Scope |
|---|---|---|
| `-reboot_recovery_timeout` | `15m` | Overall gNMI/new-boot-time recovery wait |
| `-reboot_probe_timeout` | `10s` | Each readiness RPC, including connection establishment |
| `-reboot_bgp_timeout` | `10m` | Each BGP session's post-reboot establishment wait |

These are upper bounds, with immediate progress when the required state is
observed. Initial BGP setup and other AFT suites retain their existing session
timeouts. Route-count requirements, the AFT scale synchronization limit and
the policy-change convergence checks are unchanged. If reboot or subsequent
BGP readiness fails, the reboot case fails and the dependent scale/VRF cases
are skipped rather than producing cascading failures.

AFT-6.3 checks collector stopping conditions every five seconds. This avoids
scheduling the first check at the same instant as the two-minute subscription
deadline; subscription, scale and policy-change deadlines remain unchanged.
The native IPv6 neighbor fallback recognizes XR's `Te`, `Hu` and `FH` interface
abbreviations and still requires the exact address, complete interface suffix
and expected MAC in the same neighbor record.

The readiness and neighbor-identity unit cases run without a DUT or ATE:

```sh
go test ./feature/afts/filtered_streaming/otg_tests/afts_prefix_filtering_resilience/reboot_recovery_test.go
go test ./feature/afts/filtered_streaming/otg_tests/afts_prefix_filtering_resilience/neighbor_identity_test.go
```

## Validation

`go test ./internal/aft_binding_contract` checks the generated wire paths,
single-RPC dual-stack updates, JSON_IETF values, target preservation,
CONFIG/STATE Get types, atomic filter-plus-policy deletion, and gRPC error
propagation without connecting to a DUT or ATE.

All four suites compiled from the prepared public checkout with the public
dependency pin. The typed wire-contract test and a runtime check of the
existing SIM context multiplier passed. The Go entrypoint used by the B4
Jenkins pipeline also resolved the pinned public dependency successfully with
Go 1.26.3.

Simulator runs using matching generated binding and AFT source produced:

| Suite | Result |
|---|---|
| AFT-6.1 | PASS on 2026-11-20 from the prepared public checkout: all seven groups and four prefix-policy variants |
| AFT-6.2 | Existing PASS: dual-stack filtering and policy-swap withdrawal |
| AFT-6.3 | Existing PASS: cold reboot, scale, and per-network-instance filtering |
| AFT-6.4 | Rerun pending; the initial attempt stopped during BGP setup |

AFT-6.3 verified matching typed CONFIG/STATE values before and after a real
cold reboot, with approximately 152 seconds of reboot recovery. All six scale
cases passed. The simulator environment used IOS XR 26.4.1.34I and OTG
1.48.0-5. Its existing harness overlay supplies a ten-second pre-traffic delay
and doubles context deadlines. These results qualify that environment; they
are simulator validation. Only AFT-6.1 was rerun from the prepared public
checkout; AFT-6.2/6.3 retain their previous passing evidence with matching AFT
and generated binding source. A Jenkins build has not been submitted for this
publication.

The change also carries the AFT test fixes required by the tested source:
policy-transition/readback checks, refreshed connections after reboot,
per-network-instance setup, dynamic neighbor resolution, failure diagnostics,
and AFT cache deletion handling. AFT-6.4 still needs simulator validation after
verified interface and BGP bring-up.
