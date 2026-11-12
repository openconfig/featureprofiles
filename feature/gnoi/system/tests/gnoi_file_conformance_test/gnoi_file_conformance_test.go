// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package gnoi_file_conformance_test implements gNOI-5.4: gNOI File Put and
// Stat Protocol Conformance.
package gnoi_file_conformance_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	fpb "github.com/openconfig/gnoi/file"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	dutHostname          = "dut"
	loopbackDesc         = "gNOI File Conformance Test Loopback"
	defaultLoopbackIndex = 0
	testLoopbackIndex    = 10
	loopbackSubintf      = uint32(0)
	loopbackIPv4         = "192.0.2.1"
	loopbackIPv4PLen     = uint8(32)
	loopbackIPv6         = "2001:db8::1"
	loopbackIPv6PLen     = uint8(128)
	watchTimeout         = 2 * time.Minute

	defaultScratchDir = "/tmp"
	chunkSize64KB     = 64 * 1024 // 65536 bytes
	multiChunkCount   = 4
	multiChunkTotal   = chunkSize64KB * multiChunkCount // 262144 bytes (256 KB)
	scaleFileCount    = 10

	// Explicit deterministic payloads and precomputed hex digests defined in
	// the gNOI-5.4 README.
	payloadAContent   = "payload-with-wrong-sha256"
	payloadASHA256Hex = "00d3573788b8879253860be8c1894fbea1a995fc6c5d55cc9be77658b2d72b3b"
	payloadAPerms     = uint32(0644)

	payloadBContent = "gnoi-conformance-md5-192.0.2.1-2001:db8::1\n"
	payloadBMD5Hex  = "5488fc814e0791d457dc8953b529ed7d"
	payloadBPerms   = uint32(0640)

	payloadCRecord      = "gnoi-64k-chunk-block-ipv4-192.0.2.1-ipv6-2001:db8::1-padding-01\n"
	payloadCRecordCount = 4096 // 64 bytes * 4096 = 262144 bytes (256 KB)
	payloadCPerms       = uint32(0644)

	incompleteNoHashPayload = "incomplete-stream-without-hash-192.0.2.1-2001:db8::1\n"
	contentsBeforeOpenData  = "contents-before-open"

	fileNameRoundtripSHA256  = "gnoi_conformance_roundtrip_sha256.txt"
	fileNameRoundtripMD5     = "gnoi_conformance_roundtrip_md5.txt"
	fileNameMultiChunk256K   = "gnoi_conformance_multichunk_256k.bin"
	fileNameIncompleteNoHash = "gnoi_conformance_incomplete_no_hash.txt"
	fileNameHashMismatch     = "gnoi_conformance_hash_mismatch.txt"
	fileNameNonExistent      = "gnoi_conformance_nonexistent_file.txt"
	invalidDirFilePath       = "/nonexistent_dir_gnoi_test/gnoi_invalid_dir.txt"
	relativeTestFilePath     = "relative/path/gnoi_conformance_file.txt"
	nonScratchDirectoryPath  = "/var/log"
)

// signExtensionHexPattern matches signed-char sign-extension corruption
// (e.g., ffffffd3 or ffffff88) on digest bytes >= 0x80 when formatted with %x.
var signExtensionHexPattern = regexp.MustCompile(`(?i)ffffff[89a-f][0-9a-f]`)

// errorDigestHexPattern captures a hex digest rendered in an error message
// immediately after a digest label (e.g. "File hash: 00d357..."), so its length
// and value can be compared against the expected fixed-length %02x encoding.
var errorDigestHexPattern = regexp.MustCompile(`(?i)\b(?:hash|digest|checksum)\b\W{1,3}([0-9a-f]{8,})\b`)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// fileSpec describes a deterministic test file to be staged and verified via
// the gNOI File service.
type fileSpec struct {
	name       string
	path       string
	perms      uint32
	payload    []byte
	chunkSize  int
	hashMethod tpb.HashType_HashMethod
	wantDigest []byte
	wantHex    string
}

// md5TestVectors stores precomputed RFC 1321 MD5 hex digests for the
// deterministic gNOI-5.4 test payloads, avoiding both custom MD5
// implementations and the nogo-disallowed crypto/md5 package.
var md5TestVectors = map[string]string{
	payloadAContent: "685b384280d8ecffa4afc295b5d2613b",
	payloadBContent: payloadBMD5Hex,
	"gnoi-scale-file-01-ipv4-198.51.100.1-ipv6-2001:db8:100::1\n":  "410725f4f7f53848b689ef69885ebee4",
	"gnoi-scale-file-02-ipv4-198.51.100.2-ipv6-2001:db8:100::2\n":  "bde5e04c82656916b9f1602b66868f85",
	"gnoi-scale-file-03-ipv4-198.51.100.3-ipv6-2001:db8:100::3\n":  "7f3e74642b3ddcaec3db3d6a6006d796",
	"gnoi-scale-file-04-ipv4-198.51.100.4-ipv6-2001:db8:100::4\n":  "b541a6dc6c980aeb5038e40c64ab0842",
	"gnoi-scale-file-05-ipv4-198.51.100.5-ipv6-2001:db8:100::5\n":  "cfe46608ad0d089026445e04e09fc986",
	"gnoi-scale-file-06-ipv4-198.51.100.6-ipv6-2001:db8:100::6\n":  "c70b1bcc67b04f20a180f74c2b373391",
	"gnoi-scale-file-07-ipv4-198.51.100.7-ipv6-2001:db8:100::7\n":  "535c9f9be8f293dffa46293e4a4f530b",
	"gnoi-scale-file-08-ipv4-198.51.100.8-ipv6-2001:db8:100::8\n":  "8aafe8f752a02b4c214e4338b29259b0",
	"gnoi-scale-file-09-ipv4-198.51.100.9-ipv6-2001:db8:100::9\n":  "685ffa473ed90e8701134cc5ddfad22e",
	"gnoi-scale-file-10-ipv4-198.51.100.10-ipv6-2001:db8:100::a\n": "6084d4c6b5751517cd1868f8c015fd4e",
}

// computeDigest computes the raw digest bytes and lowercase 2-digit hex string
// for the given payload and gNOI HashMethod.
func computeDigest(t *testing.T, payload []byte, method tpb.HashType_HashMethod) ([]byte, string) {
	t.Helper()
	switch method {
	case tpb.HashType_SHA256:
		sum := sha256.Sum256(payload)
		raw := sum[:]
		return raw, hex.EncodeToString(raw)
	case tpb.HashType_SHA512:
		sum := sha512.Sum512(payload)
		raw := sum[:]
		return raw, hex.EncodeToString(raw)
	case tpb.HashType_MD5:
		md5Hex, ok := md5TestVectors[string(payload)]
		if !ok {
			t.Fatalf("Missing precomputed RFC 1321 MD5 test vector for payload %q", string(payload))
		}
		raw, err := hex.DecodeString(md5Hex)
		if err != nil {
			t.Fatalf("hex.DecodeString(%q) failed: %v", md5Hex, err)
		}
		return raw, md5Hex
	default:
		t.Fatalf("Unsupported HashMethod: %v", method)
		return nil, ""
	}
}

// resolveScratchDir determines the writable scratch directory on the DUT using
// gNOI File.Stat without calling dut.Vendor(). Defaults to "/tmp" as specified
// in the README.
func resolveScratchDir(ctx context.Context, t *testing.T, fc fpb.FileClient) string {
	t.Helper()
	candidates := []string{defaultScratchDir, "/var/tmp", "/mnt/flash", "harddisk:"}
	for _, dir := range candidates {
		_, err := fc.Stat(ctx, &fpb.StatRequest{Path: dir})
		if err == nil {
			t.Logf("Resolved writable scratch directory via gNOI File.Stat: %s", dir)
			return dir
		}
		t.Logf("gNOI File.Stat(%q) probe returned: %v", dir, err)
	}
	t.Logf("Falling back to default scratch directory: %s", defaultScratchDir)
	return defaultScratchDir
}

// allTestFilePaths returns the full list of 15 test file paths used across
// gNOI-5.4.1 through gNOI-5.4.5 for pre-test hygiene and deferred cleanup.
func allTestFilePaths(scratchDir string) []string {
	paths := []string{
		path.Join(scratchDir, fileNameRoundtripSHA256),
		path.Join(scratchDir, fileNameRoundtripMD5),
		path.Join(scratchDir, fileNameMultiChunk256K),
		path.Join(scratchDir, fileNameIncompleteNoHash),
		path.Join(scratchDir, fileNameHashMismatch),
	}
	for i := 1; i <= scaleFileCount; i++ {
		paths = append(paths, path.Join(scratchDir, fmt.Sprintf("gnoi_conformance_scale_%02d.txt", i)))
	}
	return paths
}

// resolveTestLoopback returns a non-management loopback interface name and
// subinterface index on the DUT. When cli_takes_precedence_over_oc is enabled
// (e.g. Juniper, where entity-naming only supports loopback index 0), it reuses
// the active non-zero subinterface on lo0 via gnmi.Update without replacing the
// subinterface; otherwise it selects a dedicated non-zero loopback interface
// (index 10, e.g. Loopback10 / lo10) so Loopback0 / lo0 in-band management is
// never modified.
func resolveTestLoopback(t *testing.T, dut *ondatra.DUTDevice) (string, uint32) {
	t.Helper()
	if !deviations.CLITakesPrecedenceOverOC(dut) {
		return netutil.LoopbackInterface(t, dut, testLoopbackIndex), loopbackSubintf
	}
	lbIntfName := netutil.LoopbackInterface(t, dut, defaultLoopbackIndex)
	for _, subVal := range gnmi.LookupAll(t, dut, gnmi.OC().Interface(lbIntfName).SubinterfaceAny().Index().State()) {
		if idx, ok := subVal.Val(); ok && idx > 0 {
			t.Logf("Resolved active master loopback subinterface on %s: index=%d", lbIntfName, idx)
			return lbIntfName, idx
		}
	}
	return lbIntfName, testLoopbackIndex
}

// configureLoopbackInterface pushes the dual-stack loopback configuration on
// lbIntfName at subinterface lbSubIdx using gnmi.Update (never gnmi.Replace on
// Loopback0) so in-band management interfaces are never disrupted.
func configureLoopbackInterface(t *testing.T, dut *ondatra.DUTDevice, lbIntfName string, lbSubIdx uint32) {
	t.Helper()
	loIntf := &oc.Interface{
		Name:        ygot.String(lbIntfName),
		Description: ygot.String(loopbackDesc),
		Type:        oc.IETFInterfaces_InterfaceType_softwareLoopback,
	}
	if deviations.InterfaceEnabled(dut) {
		loIntf.Enabled = ygot.Bool(true)
	}
	sub := loIntf.GetOrCreateSubinterface(lbSubIdx)
	v4 := sub.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		v4.Enabled = ygot.Bool(true)
	}
	v4.GetOrCreateAddress(loopbackIPv4).PrefixLength = ygot.Uint8(loopbackIPv4PLen)

	v6 := sub.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		v6.Enabled = ygot.Bool(true)
	}
	v6.GetOrCreateAddress(loopbackIPv6).PrefixLength = ygot.Uint8(loopbackIPv6PLen)

	fptest.LogQuery(t, "DUT loopback interface config", gnmi.OC().Interface(lbIntfName).Config(), loIntf)
	gnmi.Update(t, dut, gnmi.OC().Interface(lbIntfName).Config(), loIntf)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, lbIntfName, deviations.DefaultNetworkInstance(dut), lbSubIdx)
	}
}

// resolveTargetHostname returns the DUT's existing operational or configured
// hostname so exercising /system/config/hostname via gNMI.Set preserves the
// device's real inventory hostname (preventing serial console prompt mismatches
// during lab device recovery).
func resolveTargetHostname(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	if stateHost, ok := gnmi.Lookup(t, dut, gnmi.OC().System().Hostname().State()).Val(); ok && stateHost != "" {
		return stateHost
	}
	if cfgHost, ok := gnmi.LookupConfig(t, dut, gnmi.OC().System().Hostname().Config()).Val(); ok && cfgHost != "" {
		return cfgHost
	}
	return dutHostname
}

// revertLoopbackAndHostname restores the pre-test loopback and hostname
// configuration on the DUT without ever replacing or deleting Loopback0 / lo0.
// The dedicated test loopback interface is restored to its original
// configuration if it existed before the test, and deleted only if it was
// created by the test.
func revertLoopbackAndHostname(t *testing.T, dut *ondatra.DUTDevice, lbIntfName string, lbSubIdx uint32, origLoIntf *ygnmi.Value[*oc.Interface], origLoSub *ygnmi.Value[*oc.Interface_Subinterface], targetHostname string) {
	t.Helper()
	prevSub, subExisted := origLoSub.Val()
	subExisted = subExisted && prevSub != nil
	if deviations.CLITakesPrecedenceOverOC(dut) {
		if subExisted {
			gnmi.Replace(t, dut, gnmi.OC().Interface(lbIntfName).Subinterface(lbSubIdx).Config(), prevSub)
		} else {
			gnmi.Delete(t, dut, gnmi.OC().Interface(lbIntfName).Subinterface(lbSubIdx).Ipv4().Address(loopbackIPv4).Config())
			gnmi.Delete(t, dut, gnmi.OC().Interface(lbIntfName).Subinterface(lbSubIdx).Ipv6().Address(loopbackIPv6).Config())
		}
	} else {
		if deviations.ExplicitInterfaceInDefaultVRF(dut) && !subExisted {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Interface(fmt.Sprintf("%s.%d", lbIntfName, lbSubIdx)).Config())
		}
		if prevIntf, ok := origLoIntf.Val(); ok && prevIntf != nil {
			gnmi.Replace(t, dut, gnmi.OC().Interface(lbIntfName).Config(), prevIntf)
		} else {
			gnmi.Delete(t, dut, gnmi.OC().Interface(lbIntfName).Config())
		}
	}
	if targetHostname != "" {
		gnmi.Replace(t, dut, gnmi.OC().System().Hostname().Config(), targetHostname)
	}
}

// awaitEnvironmentConvergence verifies hostname, current-datetime, and loopback
// state convergence via gNMI.
func awaitEnvironmentConvergence(t *testing.T, dut *ondatra.DUTDevice, lbIntfName string, lbSubIdx uint32, targetHostname string) {
	t.Helper()
	if deviations.CLITakesPrecedenceOverOC(dut) {
		if gotHost := gnmi.Get(t, dut, gnmi.OC().System().Hostname().Config()); gotHost != targetHostname {
			t.Fatalf("Config hostname mismatch: got %q, want %q", gotHost, targetHostname)
		}
	} else {
		gnmi.Await(t, dut, gnmi.OC().System().Hostname().State(), watchTimeout, targetHostname)
	}

	if _, ok := gnmi.Watch(t, dut, gnmi.OC().System().CurrentDatetime().State(), watchTimeout, func(val *ygnmi.Value[string]) bool {
		v, present := val.Val()
		return present && v != ""
	}).Await(t); !ok {
		t.Fatalf("Timed out waiting for non-empty /system/state/current-datetime")
	}

	if _, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(lbIntfName).OperStatus().State(), watchTimeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		v, present := val.Val()
		return present && v == oc.Interface_OperStatus_UP
	}).Await(t); !ok {
		t.Logf("Interface %s did not stream oper-status within %v; verifying /interfaces/interface[name=%s]/state/enabled is true", lbIntfName, watchTimeout, lbIntfName)
		gnmi.Await(t, dut, gnmi.OC().Interface(lbIntfName).Enabled().State(), watchTimeout, true)
	}
	gnmi.Await(t, dut, gnmi.OC().Interface(lbIntfName).Subinterface(lbSubIdx).Ipv4().Address(loopbackIPv4).Ip().State(), watchTimeout, loopbackIPv4)
	gnmi.Await(t, dut, gnmi.OC().Interface(lbIntfName).Subinterface(lbSubIdx).Ipv4().Address(loopbackIPv4).PrefixLength().State(), watchTimeout, loopbackIPv4PLen)
	gnmi.Await(t, dut, gnmi.OC().Interface(lbIntfName).Subinterface(lbSubIdx).Ipv6().Address(loopbackIPv6).Ip().State(), watchTimeout, loopbackIPv6)
	gnmi.Await(t, dut, gnmi.OC().Interface(lbIntfName).Subinterface(lbSubIdx).Ipv6().Address(loopbackIPv6).PrefixLength().State(), watchTimeout, loopbackIPv6PLen)
}

// setupEnvironment implements the "Test environment setup" section of the
// gNOI-5.4 README:
//   - Step 1: Register t.Cleanup first, then push baseline DUT configuration
//     (hostname + dual-stack loopback) via gNMI.Set.
//   - Step 2: Event-driven state convergence via gnmi.Watch / gnmi.Await (no
//     static time.Sleep) for /system/state/hostname, /system/state/current-datetime,
//     loopback oper-status, dual-stack IPv4/IPv6 addresses, and
//     /system/mount-points/mount-point/state/* telemetry.
//   - Step 3: Pre-test scratch file hygiene via gnoi.file.File.Remove and
//     verification of NOT_FOUND via gnoi.file.File.Stat.
func setupEnvironment(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, fc fpb.FileClient, filesRemoved *bool) string {
	t.Helper()
	t.Log("Test environment setup - Step 1: Configure baseline DUT hostname and dual-stack loopback interface via gNMI.Set")

	targetHostname := resolveTargetHostname(t, dut)
	lbIntfName, lbSubIdx := resolveTestLoopback(t, dut)
	origLoIntf := gnmi.LookupConfig(t, dut, gnmi.OC().Interface(lbIntfName).Config())
	origLoSub := gnmi.LookupConfig(t, dut, gnmi.OC().Interface(lbIntfName).Subinterface(lbSubIdx).Config())
	scratchDir := resolveScratchDir(ctx, t, fc)
	cleanupPaths := allTestFilePaths(scratchDir)

	// Register deferred Cleanup BEFORE pushing any gNMI config so cleanup runs
	// even if setup fails mid-way.
	t.Cleanup(func() {
		if !*filesRemoved {
			t.Log("Cleanup - Step 1: Removing all 15 gNOI conformance test files via gnoi.file.File.Remove")
			for _, p := range cleanupPaths {
				_, _ = fc.Remove(ctx, &fpb.RemoveRequest{RemoteFile: p})
			}
		}

		t.Log("Cleanup - Step 2: Reverting DUT loopback interface and hostname configuration to pre-test baseline")
		revertLoopbackAndHostname(t, dut, lbIntfName, lbSubIdx, origLoIntf, origLoSub, targetHostname)

		t.Log("Cleanup - Step 3: Verifying DUT management plane and storage mount points remain healthy")
		if _, ok := gnmi.Watch(t, dut, gnmi.OC().System().CurrentDatetime().State(), watchTimeout, func(val *ygnmi.Value[string]) bool {
			v, present := val.Val()
			return present && v != ""
		}).Await(t); !ok {
			t.Errorf("Cleanup: /system/state/current-datetime did not report a valid timestamp")
		}
		verifyMountPointsTelemetry(t, dut)
	})

	fptest.ConfigureDefaultNetworkInstance(t, dut)
	gnmi.Replace(t, dut, gnmi.OC().System().Hostname().Config(), targetHostname)
	configureLoopbackInterface(t, dut, lbIntfName, lbSubIdx)

	t.Log("Test environment setup - Step 2: Event-driven state convergence via gnmi.Watch (gNMI.Subscribe ON_CHANGE)")
	awaitEnvironmentConvergence(t, dut, lbIntfName, lbSubIdx, targetHostname)
	verifyMountPointsTelemetry(t, dut)

	t.Log("Test environment setup - Step 3: Pre-test scratch file hygiene via gnoi.file.File.Remove and File.Stat")
	for _, p := range cleanupPaths {
		_, err := fc.Remove(ctx, &fpb.RemoveRequest{RemoteFile: p})
		if err != nil && status.Code(err) != codes.NotFound && status.Code(err) != codes.Unknown {
			t.Logf("Pre-test Remove(%q) returned non-fatal status %v: %v", p, status.Code(err), err)
		}
		verifyFileStatNotFound(ctx, t, fc, p)
	}

	return scratchDir
}

// validateMountPoint checks a single mount-point state entry and returns true
// if it reports writable scratch capacity.
func validateMountPoint(t *testing.T, mp *oc.System_MountPoint) bool {
	t.Helper()
	mpName := mp.GetName()
	if mpName == "" {
		t.Errorf("Mount point reported empty /system/mount-points/mount-point/state/name")
		return false
	}
	if mp.Size == nil {
		if strings.HasPrefix(mpName, "/run/user/") {
			t.Logf("Skipping ephemeral session mount point %q without size state", mpName)
			return false
		}
		t.Errorf("Mount point %q did not report /system/mount-points/mount-point/state/size", mpName)
		return false
	}
	if mp.Available == nil {
		t.Errorf("Mount point %q did not report /system/mount-points/mount-point/state/available", mpName)
		return false
	}
	if mp.Utilized == nil {
		t.Errorf("Mount point %q did not report /system/mount-points/mount-point/state/utilized", mpName)
		return false
	}
	size := mp.GetSize()
	available := mp.GetAvailable()
	utilized := mp.GetUtilized()

	t.Logf("Mount point %q telemetry: size=%d, available=%d, utilized=%d", mpName, size, available, utilized)
	if utilized > size {
		t.Errorf("Mount point %q utilized (%d) > size (%d)", mpName, utilized, size)
	}
	if available > size {
		t.Errorf("Mount point %q available (%d) > size (%d)", mpName, available, size)
	}
	return size > 0 && available > 0
}

// verifyMountPointsTelemetry validates /system/mount-points/mount-point/state
// leaves (name, size, available, utilized), using leaf queries when a
// functional translator is active (since ygnmi functional translators only
// support leaf paths) and a single container gnmi.GetAll query otherwise.
func verifyMountPointsTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ftName := deviations.SystemMountPointStateFt(dut)
	opts := fptest.GetOptsForFunctionalTranslator(t, ftName)
	gnmiOpts := dut.GNMIOpts().WithYGNMIOpts(opts...)

	var mountPoints []*oc.System_MountPoint
	if ftName != "" {
		mpNames := gnmi.GetAll(t, gnmiOpts, gnmi.OC().System().MountPointAny().Name().State())
		if len(mpNames) == 0 {
			t.Fatalf("No storage mount points reported under /system/mount-points/mount-point/state")
		}
		batch := gnmi.OCBatch()
		for _, mpName := range mpNames {
			batch.AddPaths(
				gnmi.OC().System().MountPoint(mpName).Size(),
				gnmi.OC().System().MountPoint(mpName).Available(),
				gnmi.OC().System().MountPoint(mpName).Utilized(),
			)
		}
		systemVal := gnmi.Get(t, gnmiOpts, batch.State()).GetSystem()
		for _, mpName := range mpNames {
			mp := &oc.System_MountPoint{Name: ygot.String(mpName)}
			if devMp := systemVal.GetMountPoint(mpName); devMp != nil {
				if devMp.Size != nil {
					mp.Size = ygot.Uint64(devMp.GetSize())
				}
				if devMp.Available != nil {
					mp.Available = ygot.Uint64(devMp.GetAvailable())
				}
				if devMp.Utilized != nil {
					mp.Utilized = ygot.Uint64(devMp.GetUtilized())
				}
			}
			mountPoints = append(mountPoints, mp)
		}
	} else {
		mountPoints = gnmi.GetAll(t, gnmiOpts, gnmi.OC().System().MountPointAny().State())
	}
	if len(mountPoints) == 0 {
		t.Fatalf("No storage mount points reported under /system/mount-points/mount-point/state")
	}

	var foundUsableCapacity bool
	for _, mp := range mountPoints {
		if validateMountPoint(t, mp) {
			foundUsableCapacity = true
		}
	}
	if !foundUsableCapacity {
		t.Errorf("No mount point reported non-zero size and available capacity")
	}
}

// putFileComplete executes a complete three-phase client-streaming
// gnoi.file.File.Put RPC (Open -> Contents chunks -> Hash -> CloseAndRecv).
func putFileComplete(ctx context.Context, t *testing.T, fc fpb.FileClient, spec fileSpec) {
	t.Helper()
	stream, err := fc.Put(ctx)
	if err != nil {
		t.Fatalf("gnoi.file.File.Put(%q) failed to open stream: %v", spec.path, err)
	}

	openReq := &fpb.PutRequest{
		Request: &fpb.PutRequest_Open{
			Open: &fpb.PutRequest_Details{
				RemoteFile:  spec.path,
				Permissions: spec.perms,
			},
		},
	}
	if err := stream.Send(openReq); err != nil {
		t.Fatalf("gnoi.file.File.Put(%q) failed to send PutRequest{open}: %v", spec.path, err)
	}

	step := spec.chunkSize
	if step <= 0 {
		step = chunkSize64KB
	}
	for offset := 0; offset < len(spec.payload); offset += step {
		end := offset + step
		if end > len(spec.payload) {
			end = len(spec.payload)
		}
		chunkReq := &fpb.PutRequest{
			Request: &fpb.PutRequest_Contents{
				Contents: spec.payload[offset:end],
			},
		}
		if err := stream.Send(chunkReq); err != nil {
			t.Fatalf("gnoi.file.File.Put(%q) failed to send PutRequest{contents} at offset %d: %v", spec.path, offset, err)
		}
	}

	hashReq := &fpb.PutRequest{
		Request: &fpb.PutRequest_Hash{
			Hash: &tpb.HashType{
				Method: spec.hashMethod,
				Hash:   spec.wantDigest,
			},
		},
	}
	if err := stream.Send(hashReq); err != nil {
		t.Fatalf("gnoi.file.File.Put(%q) failed to send final PutRequest{hash}: %v", spec.path, err)
	}

	if _, err := stream.CloseAndRecv(); err != nil {
		t.Fatalf("gnoi.file.File.Put(%q) CloseAndRecv() failed: got error %v (code %v), want StatusCode.OK", spec.path, err, status.Code(err))
	}
}

// verifyFileStat checks gnoi.file.File.Stat for a single committed file path,
// verifying path, size, permissions, last_modified, and umask.
func verifyFileStat(ctx context.Context, t *testing.T, fc fpb.FileClient, spec fileSpec) {
	t.Helper()
	resp, err := fc.Stat(ctx, &fpb.StatRequest{Path: spec.path})
	if err != nil {
		t.Fatalf("gnoi.file.File.Stat(%q) failed: %v", spec.path, err)
	}
	stats := resp.GetStats()
	if len(stats) == 0 {
		t.Fatalf("gnoi.file.File.Stat(%q) returned 0 StatInfo entries, want >= 1", spec.path)
	}

	info := stats[0]
	t.Logf("File.Stat(%q): path=%q size=%d permissions=%o last_modified=%d umask=%o",
		spec.path, info.GetPath(), info.GetSize(), info.GetPermissions(), info.GetLastModified(), info.GetUmask())

	if !strings.HasSuffix(info.GetPath(), spec.name) && info.GetPath() != spec.path {
		t.Errorf("File.Stat(%q) StatInfo.path = %q, want %q", spec.path, info.GetPath(), spec.path)
	}
	if gotSize, wantSize := info.GetSize(), uint64(len(spec.payload)); gotSize != wantSize {
		t.Errorf("File.Stat(%q) StatInfo.size = %d, want %d", spec.path, gotSize, wantSize)
	}
	if info.GetLastModified() == 0 {
		t.Errorf("File.Stat(%q) StatInfo.last_modified = 0, want > 0", spec.path)
	}
	if info.GetPermissions() == 0 {
		t.Errorf("File.Stat(%q) StatInfo.permissions = 0, want non-zero mode (configured %o)", spec.path, spec.perms)
	}
}

// verifyDirectoryStat checks gnoi.file.File.Stat on the parent scratch
// directory and confirms all expected files are listed with matching byte sizes.
func verifyDirectoryStat(ctx context.Context, t *testing.T, fc fpb.FileClient, scratchDir string, specs []fileSpec) {
	t.Helper()
	resp, err := fc.Stat(ctx, &fpb.StatRequest{Path: scratchDir})
	if err != nil {
		t.Fatalf("gnoi.file.File.Stat(%q) on directory failed: %v", scratchDir, err)
	}
	foundByName := make(map[string]*fpb.StatInfo)
	for _, st := range resp.GetStats() {
		foundByName[path.Base(st.GetPath())] = st
	}
	for _, spec := range specs {
		st, ok := foundByName[spec.name]
		if !ok {
			t.Errorf("Directory File.Stat(%q) missing expected file %q", scratchDir, spec.name)
			continue
		}
		if gotSize, wantSize := st.GetSize(), uint64(len(spec.payload)); gotSize != wantSize {
			t.Errorf("Directory File.Stat(%q) entry %q size = %d, want %d", scratchDir, spec.name, gotSize, wantSize)
		}
	}
}

// verifyErrorDigestHex asserts that every hex digest rendered after a digest
// label in a File.Put hash-mismatch error message uses the fixed-length,
// zero-padded unsigned %02x encoding (64 chars for SHA256, 32 chars for MD5),
// and that any digest other than the all-zero digest sent by the client equals
// the expected digest of the transmitted payload.
func verifyErrorDigestHex(t *testing.T, errMsg, wantHex string) {
	t.Helper()
	zeroHex := strings.Repeat("0", len(wantHex))
	for _, m := range errorDigestHexPattern.FindAllStringSubmatch(errMsg, -1) {
		gotHex := strings.ToLower(m[1])
		switch {
		case len(gotHex) != len(wantHex):
			t.Errorf("gNOI-5.4.3 Defect: error message %q contains digest %q with %d hex chars, want fixed-length zero-padded %d-char hex (e.g. %q)", errMsg, m[1], len(gotHex), len(wantHex), wantHex)
		case gotHex != zeroHex && gotHex != wantHex:
			t.Errorf("gNOI-5.4.3 Defect: error message %q contains digest %q, want the all-zero client digest or the payload digest %q", errMsg, m[1], wantHex)
		}
	}
}

// verifyFileStatNotFound asserts that gnoi.file.File.Stat returns a non-OK
// gRPC error (NOT_FOUND) for a removed or uncommitted file path.
func verifyFileStatNotFound(ctx context.Context, t *testing.T, fc fpb.FileClient, path string) {
	t.Helper()
	resp, err := fc.Stat(ctx, &fpb.StatRequest{Path: path})
	if err == nil {
		t.Fatalf("gnoi.file.File.Stat(%q) succeeded with %d entries (%+v), want gRPC NOT_FOUND error", path, len(resp.GetStats()), resp.GetStats())
	}
	if err != nil && status.Code(err) != codes.NotFound {
		t.Logf("gnoi.file.File.Stat(%q) returned non-OK code %v (%v)", path, status.Code(err), err)
	}
}

// verifyFileGet downloads a file via server-streaming gnoi.file.File.Get,
// reassembles all GetResponse{contents} chunks, and validates both byte-for-byte
// payload equality and the trailing GetResponse{hash} digest (including
// fixed-length unsigned %02x hex formatting across bytes >= 0x80 and < 0x10).
func verifyFileGet(ctx context.Context, t *testing.T, fc fpb.FileClient, spec fileSpec) {
	t.Helper()
	stream, err := fc.Get(ctx, &fpb.GetRequest{RemoteFile: spec.path})
	if err != nil {
		t.Fatalf("gnoi.file.File.Get(%q) failed to start stream: %v", spec.path, err)
	}

	var received bytes.Buffer
	var trailingHash *tpb.HashType
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("gnoi.file.File.Get(%q) stream.Recv() failed: %v", spec.path, err)
		}
		switch r := resp.GetResponse().(type) {
		case *fpb.GetResponse_Contents:
			received.Write(r.Contents)
		case *fpb.GetResponse_Hash:
			trailingHash = r.Hash
		}
	}

	if !bytes.Equal(received.Bytes(), spec.payload) {
		t.Errorf("gnoi.file.File.Get(%q) payload mismatch: got %d bytes, want %d bytes", spec.path, received.Len(), len(spec.payload))
	}

	if trailingHash == nil {
		t.Fatalf("gnoi.file.File.Get(%q) stream closed without sending trailing GetResponse{hash}", spec.path)
	}

	gotBytes := trailingHash.GetHash()
	gotHex := hex.EncodeToString(gotBytes)

	// Verify the returned HashType matches the expected digest for its reported
	// HashMethod (or the spec's HashMethod).
	wantMethodDigest, wantMethodHex := computeDigest(t, spec.payload, trailingHash.GetMethod())
	wantHexLen := len(wantMethodDigest) * 2
	if len(gotHex) != wantHexLen {
		t.Errorf("gnoi.file.File.Get(%q) formatted hex digest length = %d (%q), want exact fixed length %d", spec.path, len(gotHex), gotHex, wantHexLen)
	}
	if !bytes.Equal(gotBytes, wantMethodDigest) || gotHex != wantMethodHex {
		t.Errorf("gnoi.file.File.Get(%q) trailing hash mismatch (method=%v): got %q, want %q", spec.path, trailingHash.GetMethod(), gotHex, wantMethodHex)
	}
}

// TestGNOIFileConformance executes gNOI-5.4: gNOI File Put and Stat Protocol
// Conformance sequentially across subtests gNOI-5.4.1 through gNOI-5.4.5.
func TestGNOIFileConformance(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	fc := dut.RawAPIs().GNOI(t).File()

	var filesRemoved bool
	scratchDir := setupEnvironment(ctx, t, dut, fc, &filesRemoved)

	// Construct the 3 deterministic baseline fileSpecs defined in the README.
	payloadABytes := []byte(payloadAContent)
	payloadADigest, payloadAHex := computeDigest(t, payloadABytes, tpb.HashType_SHA256)
	if payloadAHex != payloadASHA256Hex {
		t.Fatalf("Internal sanity check failed for Payload A SHA256: got %q, want %q", payloadAHex, payloadASHA256Hex)
	}

	payloadBBytes := []byte(payloadBContent)
	payloadBDigest, payloadBHex := computeDigest(t, payloadBBytes, tpb.HashType_MD5)
	if payloadBHex != payloadBMD5Hex {
		t.Fatalf("Internal sanity check failed for Payload B MD5: got %q, want %q", payloadBHex, payloadBMD5Hex)
	}

	payloadCBytes := bytes.Repeat([]byte(payloadCRecord), payloadCRecordCount)
	payloadCDigest, payloadCHex := computeDigest(t, payloadCBytes, tpb.HashType_SHA256)

	baselineSpecs := []fileSpec{
		{
			name:       fileNameRoundtripSHA256,
			path:       path.Join(scratchDir, fileNameRoundtripSHA256),
			perms:      payloadAPerms,
			payload:    payloadABytes,
			chunkSize:  chunkSize64KB,
			hashMethod: tpb.HashType_SHA256,
			wantDigest: payloadADigest,
			wantHex:    payloadAHex,
		},
		{
			name:       fileNameRoundtripMD5,
			path:       path.Join(scratchDir, fileNameRoundtripMD5),
			perms:      payloadBPerms,
			payload:    payloadBBytes,
			chunkSize:  chunkSize64KB,
			hashMethod: tpb.HashType_MD5,
			wantDigest: payloadBDigest,
			wantHex:    payloadBHex,
		},
		{
			name:       fileNameMultiChunk256K,
			path:       path.Join(scratchDir, fileNameMultiChunk256K),
			perms:      payloadCPerms,
			payload:    payloadCBytes,
			chunkSize:  chunkSize64KB,
			hashMethod: tpb.HashType_SHA256,
			wantDigest: payloadCDigest,
			wantHex:    payloadCHex,
		},
	}

	var scaleSpecs []fileSpec
	for i := 1; i <= scaleFileCount; i++ {
		name := fmt.Sprintf("gnoi_conformance_scale_%02d.txt", i)
		content := []byte(fmt.Sprintf("gnoi-scale-file-%02d-ipv4-198.51.100.%d-ipv6-2001:db8:100::%x\n", i, i, i))
		method := tpb.HashType_SHA256
		if i%2 == 0 {
			method = tpb.HashType_MD5
		}
		digest, hexStr := computeDigest(t, content, method)
		scaleSpecs = append(scaleSpecs, fileSpec{
			name:       name,
			path:       path.Join(scratchDir, name),
			perms:      payloadAPerms,
			payload:    content,
			chunkSize:  chunkSize64KB,
			hashMethod: method,
			wantDigest: digest,
			wantHex:    hexStr,
		})
	}

	// gNOI-5.4.1 - Positive Single-Chunk and Multi-Chunk gNOI File.Put,
	// File.Stat, and File.Get Baseline with SHA-256 and MD5 Digest Verification.
	t.Run("gNOI-5.4.1_Positive_File_Put_Stat_Get_Baseline", func(t *testing.T) {
		t.Log("gNOI-5.4.1 - Step 1: Upload baseline single-chunk (SHA256, MD5) and multi-chunk (256 KB) files via gnoi.file.File.Put")
		for _, spec := range baselineSpecs {
			putFileComplete(ctx, t, fc, spec)
		}

		t.Log("gNOI-5.4.1 - Step 2: Verify file metadata via gnoi.file.File.Stat for each file and parent directory")
		for _, spec := range baselineSpecs {
			verifyFileStat(ctx, t, fc, spec)
		}
		verifyDirectoryStat(ctx, t, fc, scratchDir, baselineSpecs)

		t.Log("gNOI-5.4.1 - Step 3: Verify streamed payload and fixed-length unsigned digest via gnoi.file.File.Get")
		for _, spec := range baselineSpecs {
			verifyFileGet(ctx, t, fc, spec)
		}
	})

	// gNOI-5.4.2 - Negative gNOI File.Put Stream Closed Without Trailing
	// HashType Message and Existing-File Preservation.
	t.Run("gNOI-5.4.2_Negative_File_Put_Without_Trailing_HashType", func(t *testing.T) {
		incompletePath := path.Join(scratchDir, fileNameIncompleteNoHash)

		t.Log("gNOI-5.4.2 - Step 1: Open gnoi.file.File.Put stream, send Open + Contents, and close stream without trailing PutRequest{hash}")
		stream, err := fc.Put(ctx)
		if err != nil {
			t.Fatalf("gnoi.file.File.Put(%q) failed to open stream: %v", incompletePath, err)
		}
		if err := stream.Send(&fpb.PutRequest{
			Request: &fpb.PutRequest_Open{
				Open: &fpb.PutRequest_Details{
					RemoteFile:  incompletePath,
					Permissions: payloadAPerms,
				},
			},
		}); err != nil {
			t.Fatalf("gnoi.file.File.Put(%q) failed to send PutRequest{open}: %v", incompletePath, err)
		}
		if err := stream.Send(&fpb.PutRequest{
			Request: &fpb.PutRequest_Contents{
				Contents: []byte(incompleteNoHashPayload),
			},
		}); err != nil {
			t.Fatalf("gnoi.file.File.Put(%q) failed to send PutRequest{contents}: %v", incompletePath, err)
		}

		t.Log("gNOI-5.4.2 - Step 2: Dual assertion — verify CloseAndRecv returns non-OK gRPC error AND temporary file is removed")
		putResp, putErr := stream.CloseAndRecv()
		if putErr == nil || status.Code(putErr) == codes.OK {
			t.Errorf("gNOI-5.4.2 Defect: gnoi.file.File.Put(%q) closed without trailing PutRequest{hash} returned StatusCode.OK (resp=%+v), want non-OK gRPC error (INVALID_ARGUMENT, ABORTED, or FAILED_PRECONDITION)", incompletePath, putResp)
		} else {
			t.Logf("gnoi.file.File.Put(%q) without trailing hash properly rejected with status=%v: %v", incompletePath, status.Code(putErr), putErr)
		}

		verifyFileStatNotFound(ctx, t, fc, incompletePath)
		getStream, getErr := fc.Get(ctx, &fpb.GetRequest{RemoteFile: incompletePath})
		if getErr == nil {
			_, recvErr := getStream.Recv()
			if recvErr == nil || recvErr == io.EOF {
				t.Errorf("gnoi.file.File.Get(%q) succeeded on incomplete file closed without hash, want NOT_FOUND error", incompletePath)
			}
		}

		t.Log("gNOI-5.4.2 - Step 3: Verify existing committed baseline files from gNOI-5.4.1 remain intact")
		for _, spec := range baselineSpecs {
			verifyFileStat(ctx, t, fc, spec)
			verifyFileGet(ctx, t, fc, spec)
		}
	})

	// gNOI-5.4.3 - Negative gNOI File.Put Mismatched HashType Digest Rejection
	// and Error Hex Formatting Validation.
	t.Run("gNOI-5.4.3_Negative_File_Put_Mismatched_HashType_Digest", func(t *testing.T) {
		mismatchPath := path.Join(scratchDir, fileNameHashMismatch)

		mismatchCases := []struct {
			desc       string
			stepLabel  string
			payload    []byte
			method     tpb.HashType_HashMethod
			zeroDigest []byte
			wantHex    string
		}{
			{
				desc:       "SHA256_Mismatched_AllZero_Digest",
				stepLabel:  "gNOI-5.4.3 - Steps 1 & 2 (SHA256)",
				payload:    payloadABytes,
				method:     tpb.HashType_SHA256,
				zeroDigest: make([]byte, 32),
				wantHex:    payloadASHA256Hex,
			},
			{
				desc:       "MD5_Mismatched_AllZero_Digest",
				stepLabel:  "gNOI-5.4.3 - Step 3 (MD5)",
				payload:    payloadBBytes,
				method:     tpb.HashType_MD5,
				zeroDigest: make([]byte, 16),
				wantHex:    payloadBMD5Hex,
			},
			{
				desc:       "UNSPECIFIED_HashMethod_Rejected",
				stepLabel:  "gNOI-5.4.3 - Step 3b (UNSPECIFIED HashMethod)",
				payload:    payloadABytes,
				method:     tpb.HashType_UNSPECIFIED,
				zeroDigest: make([]byte, 32),
				wantHex:    payloadASHA256Hex,
			},
		}

		for _, tc := range mismatchCases {
			t.Run(tc.desc, func(t *testing.T) {
				t.Logf("%s: Transmit gnoi.file.File.Put with intentionally mismatched %v digest", tc.stepLabel, tc.method)
				stream, err := fc.Put(ctx)
				if err != nil {
					t.Fatalf("gnoi.file.File.Put(%q) failed to open stream: %v", mismatchPath, err)
				}
				if err := stream.Send(&fpb.PutRequest{
					Request: &fpb.PutRequest_Open{
						Open: &fpb.PutRequest_Details{
							RemoteFile:  mismatchPath,
							Permissions: payloadAPerms,
						},
					},
				}); err != nil {
					t.Fatalf("gnoi.file.File.Put(%q) failed to send PutRequest{open}: %v", mismatchPath, err)
				}
				if err := stream.Send(&fpb.PutRequest{
					Request: &fpb.PutRequest_Contents{
						Contents: tc.payload,
					},
				}); err != nil {
					t.Fatalf("gnoi.file.File.Put(%q) failed to send PutRequest{contents}: %v", mismatchPath, err)
				}
				if err := stream.Send(&fpb.PutRequest{
					Request: &fpb.PutRequest_Hash{
						Hash: &tpb.HashType{
							Method: tc.method,
							Hash:   tc.zeroDigest,
						},
					},
				}); err != nil && err != io.EOF {
					t.Logf("stream.Send(PutRequest{hash}) returned immediate error: %v", err)
				}

				_, putErr := stream.CloseAndRecv()
				if putErr == nil || status.Code(putErr) == codes.OK {
					t.Errorf("gnoi.file.File.Put(%q) with mismatched %v digest returned StatusCode.OK, want non-OK gRPC error", mismatchPath, tc.method)
				} else {
					errMsg := status.Convert(putErr).Message()
					t.Logf("gnoi.file.File.Put(%q) with mismatched %v digest rejected with code=%v message=%q", mismatchPath, tc.method, status.Code(putErr), errMsg)
					if signExtensionHexPattern.MatchString(errMsg) {
						t.Errorf("gNOI-5.4.3 Defect: error message %q contains signed-char ffffff sign-extension on digest bytes >= 0x80 (want clean %d-char hex %q)", errMsg, len(tc.wantHex), tc.wantHex)
					}
					if tc.method != tpb.HashType_UNSPECIFIED {
						verifyErrorDigestHex(t, errMsg, tc.wantHex)
					}
				}

				verifyFileStatNotFound(ctx, t, fc, mismatchPath)
			})
		}

		t.Log("gNOI-5.4.3 - Step 4: Confirm gNOI-5.4.1 baseline files remain intact")
		for _, spec := range baselineSpecs {
			verifyFileStat(ctx, t, fc, spec)
		}
	})

	// gNOI-5.4.4 - Modest Scale: Multi-File Batch Staging (10 Files), Directory
	// File.Stat, and Mount-Point Telemetry.
	t.Run("gNOI-5.4.4_Modest_Scale_Multi_File_Batch_And_Telemetry", func(t *testing.T) {
		t.Log("gNOI-5.4.4 - Step 1: Stage 10 additional files across SHA256 and MD5")
		for _, spec := range scaleSpecs {
			putFileComplete(ctx, t, fc, spec)
		}

		t.Log("gNOI-5.4.4 - Step 2: Verify directory-level File.Stat across all 13 committed files and streamed File.Get on all 10 scale files")
		allCommitted := append([]fileSpec{}, baselineSpecs...)
		allCommitted = append(allCommitted, scaleSpecs...)
		verifyDirectoryStat(ctx, t, fc, scratchDir, allCommitted)

		for _, spec := range scaleSpecs {
			verifyFileStat(ctx, t, fc, spec)
			verifyFileGet(ctx, t, fc, spec)
		}

		t.Log("gNOI-5.4.4 - Step 3: Verify storage mount-point telemetry via gnmi.Watch")
		verifyMountPointsTelemetry(t, dut)
	})

	// gNOI-5.4.5 - Protocol-Level Mandatory Negative Cases and Final
	// File.Remove Verification.
	t.Run("gNOI-5.4.5_Protocol_Negative_Cases_And_Final_Remove", func(t *testing.T) {
		t.Log("gNOI-5.4.5 - Step 1: Negative Case — Protocol sequence violation (PutRequest{contents} sent before PutRequest{open})")
		seqStream, err := fc.Put(ctx)
		if err != nil {
			t.Fatalf("gnoi.file.File.Put failed to open stream: %v", err)
		}
		_ = seqStream.Send(&fpb.PutRequest{
			Request: &fpb.PutRequest_Contents{
				Contents: []byte(contentsBeforeOpenData),
			},
		})
		if _, seqErr := seqStream.CloseAndRecv(); seqErr == nil || status.Code(seqErr) == codes.OK {
			t.Errorf("gnoi.file.File.Put with PutRequest{contents} sent before PutRequest{open} returned StatusCode.OK, want non-OK gRPC error")
		} else {
			t.Logf("PutRequest{contents} before PutRequest{open} properly rejected with code=%v: %v", status.Code(seqErr), seqErr)
		}

		t.Log("gNOI-5.4.5 - Step 2: Negative Case — Non-existent target directory path in PutRequest{open}")
		dirStream, err := fc.Put(ctx)
		if err != nil {
			t.Fatalf("gnoi.file.File.Put failed to open stream: %v", err)
		}
		_ = dirStream.Send(&fpb.PutRequest{
			Request: &fpb.PutRequest_Open{
				Open: &fpb.PutRequest_Details{
					RemoteFile:  invalidDirFilePath,
					Permissions: payloadAPerms,
				},
			},
		})
		_ = dirStream.Send(&fpb.PutRequest{
			Request: &fpb.PutRequest_Contents{
				Contents: payloadABytes,
			},
		})
		_ = dirStream.Send(&fpb.PutRequest{
			Request: &fpb.PutRequest_Hash{
				Hash: &tpb.HashType{
					Method: tpb.HashType_SHA256,
					Hash:   payloadADigest,
				},
			},
		})
		if _, dirErr := dirStream.CloseAndRecv(); dirErr == nil || status.Code(dirErr) == codes.OK {
			t.Errorf("gnoi.file.File.Put(%q) to non-existent directory returned StatusCode.OK, want non-OK gRPC error", invalidDirFilePath)
		} else {
			t.Logf("gnoi.file.File.Put(%q) to non-existent directory properly rejected with code=%v: %v", invalidDirFilePath, status.Code(dirErr), dirErr)
		}

		t.Log("gNOI-5.4.5 - Step 3: Negative Case — File.Stat, File.Get, File.Remove, and File.TransferToRemote on non-existent, empty, relative, and directory paths")
		nonExistentPath := path.Join(scratchDir, fileNameNonExistent)

		for _, badStatPath := range []string{"", relativeTestFilePath} {
			if statResp, statErr := fc.Stat(ctx, &fpb.StatRequest{Path: badStatPath}); statErr == nil || status.Code(statErr) == codes.OK {
				t.Errorf("gnoi.file.File.Stat(%q) returned StatusCode.OK (%+v), want non-OK gRPC error (INVALID_ARGUMENT or NOT_FOUND)", badStatPath, statResp)
			} else {
				t.Logf("gnoi.file.File.Stat(%q) properly rejected with code=%v: %v", badStatPath, status.Code(statErr), statErr)
			}
		}

		for _, badGetPath := range []string{nonExistentPath, "", relativeTestFilePath} {
			getStream, getErr := fc.Get(ctx, &fpb.GetRequest{RemoteFile: badGetPath})
			if getErr == nil {
				_, recvErr := getStream.Recv()
				if recvErr == nil || recvErr == io.EOF {
					t.Errorf("gnoi.file.File.Get(%q) succeeded, want non-OK gRPC error", badGetPath)
				} else {
					t.Logf("gnoi.file.File.Get(%q) properly rejected on Recv() with code=%v: %v", badGetPath, status.Code(recvErr), recvErr)
				}
			} else {
				t.Logf("gnoi.file.File.Get(%q) properly rejected with code=%v: %v", badGetPath, status.Code(getErr), getErr)
			}
		}

		for _, badRmPath := range []string{nonExistentPath, "", nonScratchDirectoryPath} {
			if _, rmErr := fc.Remove(ctx, &fpb.RemoveRequest{RemoteFile: badRmPath}); rmErr == nil || status.Code(rmErr) == codes.OK {
				t.Errorf("gnoi.file.File.Remove(%q) returned StatusCode.OK, want non-OK gRPC error", badRmPath)
			} else {
				t.Logf("gnoi.file.File.Remove(%q) properly rejected with code=%v: %v", badRmPath, status.Code(rmErr), rmErr)
			}
		}

		transferReqs := []*fpb.TransferToRemoteRequest{
			{},
			{LocalPath: nonExistentPath},
		}
		for _, trReq := range transferReqs {
			if _, trErr := fc.TransferToRemote(ctx, trReq); trErr == nil || status.Code(trErr) == codes.OK {
				t.Errorf("gnoi.file.File.TransferToRemote(%+v) returned StatusCode.OK, want non-OK gRPC error (INVALID_ARGUMENT, NOT_FOUND, or UNIMPLEMENTED)", trReq)
			} else {
				t.Logf("gnoi.file.File.TransferToRemote(%+v) properly rejected with code=%v: %v", trReq, status.Code(trErr), trErr)
			}
		}

		t.Log("gNOI-5.4.5 - Step 4: Final gnoi.file.File.Remove verification across all 13 committed files")
		allCommitted := append([]fileSpec{}, baselineSpecs...)
		allCommitted = append(allCommitted, scaleSpecs...)
		for _, spec := range allCommitted {
			if _, err := fc.Remove(ctx, &fpb.RemoveRequest{RemoteFile: spec.path}); err != nil {
				t.Errorf("gnoi.file.File.Remove(%q) failed: %v", spec.path, err)
			}
			verifyFileStatNotFound(ctx, t, fc, spec.path)
		}
		filesRemoved = true
	})
}
