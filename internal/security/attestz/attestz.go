// Copyright 2024 Google LLC
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

// Package attestz provides helper APIs to simplify writing attestz test cases.
// attestz.go: provides functions for attest rpcs and verification.
// cert.go: provides functions for certz rpcs and certificate creation.
// events.go: provides functions for triggering gnoi events on the dut.
// setup.go: provides functions for initial setup of attestz test cases.
package attestz

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-tpm/tpm2"
	cdpb "github.com/openconfig/attestz/proto/common_definitions"
	attestzpb "github.com/openconfig/attestz/proto/tpm_attestz"
	enrollzpb "github.com/openconfig/attestz/proto/tpm_enrollz"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/security/svid"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/testing/protocmp"
)

// ControlCard struct to hold certificates for a control card.
type ControlCard struct {
	Role       cdpb.ControlCardRole
	Name       string
	IAKCert    string
	IDevIDCert string
	OIAKCert   string
	ODevIDCert string
	MtlsCert   string
}

// Session represents grpc session used for attestz rpcs.
type Session struct {
	Conn          *grpc.ClientConn
	Peer          *peer.Peer
	EnrollzClient enrollzpb.TpmEnrollzServiceClient
	AttestzClient attestzpb.TpmAttestzServiceClient
}

var (
	chassisName     string
	activeCard      *ControlCard
	standbyCard     *ControlCard
	pcrBankHashAlgo = []attestzpb.Tpm20HashAlgo{
		attestzpb.Tpm20HashAlgo_TPM20HASH_ALGO_SHA1,
		attestzpb.Tpm20HashAlgo_TPM20HASH_ALGO_SHA256,
		attestzpb.Tpm20HashAlgo_TPM20HASH_ALGO_SHA384,
		attestzpb.Tpm20HashAlgo_TPM20HASH_ALGO_SHA512,
	}
	// PcrBankHashAlgoMap vendor supported hash algorithms for pcr bank.
	PcrBankHashAlgoMap = map[ondatra.Vendor][]attestzpb.Tpm20HashAlgo{
		ondatra.NOKIA:   {attestzpb.Tpm20HashAlgo_TPM20HASH_ALGO_SHA1, attestzpb.Tpm20HashAlgo_TPM20HASH_ALGO_SHA256},
		ondatra.ARISTA:  pcrBankHashAlgo,
		ondatra.JUNIPER: pcrBankHashAlgo,
		ondatra.CISCO:   pcrBankHashAlgo,
	}
	// PcrIndices pcr indices to be attested.
	PcrIndices = []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 23}
)

// PrettyPrint prints any type in a pretty format.
func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

// EnrollzWorkflow performs enrollment workflow for a given control card.
func (cc *ControlCard) EnrollzWorkflow(t *testing.T, dut *ondatra.DUTDevice, tc *TLSConf, vendorCaCertFile string) {
	as := tc.NewSession(t)
	defer as.Conn.Close()

	// Get vendor certs.
	resp := as.GetVendorCerts(t, ParseRoleSelection(cc.Role))
	cc.IAKCert, cc.IDevIDCert = resp.IakCert, resp.IdevidCert

	// Verify active card's cert is used for tls connection.
	t.Logf("Verifying correct cert was used for tls connection during enrollz.")
	var activeCert string
	if activeCard.MtlsCert != "" {
		activeCert = activeCard.MtlsCert
	} else if activeCard.ODevIDCert != "" {
		activeCert = activeCard.ODevIDCert
	} else {
		activeCert = activeCard.IDevIDCert
	}
	wantPeerCert, err := LoadCertificate(activeCert)
	if err != nil {
		t.Fatalf("Error parsing cert. error: %s", err)
	}
	tlsInfo := as.Peer.AuthInfo.(credentials.TLSInfo)
	gotPeerCert := tlsInfo.State.PeerCertificates[0]
	if diff := cmp.Diff(wantPeerCert, gotPeerCert); diff != "" {
		t.Errorf("Incorrect certificate used for enrollz tls session. -want,+got:\n%s", diff)
	}

	// Load vendor ca certificate.
	vendorCaPem, err := os.ReadFile(vendorCaCertFile)
	if err != nil {
		t.Fatalf("Error reading vendor cert. error: %v", err)
	}

	// Verify cert info.
	t.Logf("Verifying IDevID cert for card %v", cc.Name)
	cc.verifyVendorCert(t, dut, vendorCaPem, "idevid")
	t.Logf("Verifying IAK cert for card %v", cc.Name)
	cc.verifyVendorCert(t, dut, vendorCaPem, "iak")
	t.Logf("Verifying control card details for card %v", cc.Name)
	cc.verifyControlCardInfo(t, dut, resp.ControlCardId)

	// Generate owner certificates.
	caKey, caCert, err := svid.LoadKeyPair(tc.CaKeyFile, tc.CaCertFile)
	if err != nil {
		t.Fatalf("Could not load ca key/cert. error: %v", err)
	}
	t.Logf("Generating oIAK cert for card %v", cc.Name)
	cc.OIAKCert = GenOwnerCert(t, caKey, caCert, cc.IAKCert, nil, tc.Target)
	t.Logf("Generating oDevID cert for card %v", cc.Name)
	cc.ODevIDCert = GenOwnerCert(t, caKey, caCert, cc.IDevIDCert, nil, tc.Target)

	// Rotate owner certificates.
	as.RotateOwnerCerts(t, cc.Role, cc.OIAKCert, cc.ODevIDCert, sslProfileID)
}

// GenNonce creates a random 32 byte nonce used for attest rpc.
func GenNonce(t *testing.T) []byte {
	nonce := make([]byte, 32)
	_, err := rand.Read(nonce)
	if err != nil {
		t.Fatalf("Error generating nonce. error: %v", err)
	}
	return nonce
}

// AttestzWorkflow performs attestation workflow for a given control card.
func (cc *ControlCard) AttestzWorkflow(t *testing.T, dut *ondatra.DUTDevice, tc *TLSConf) {
	as := tc.NewSession(t)
	defer as.Conn.Close()

	for _, hashAlgo := range PcrBankHashAlgoMap[dut.Vendor()] {
		nonce := GenNonce(t)
		attestResponse := as.RequestAttestation(t, cc.Role, nonce, hashAlgo, PcrIndices)

		// Verify active card's cert is used for tls connection.
		t.Logf("Verifying correct cert was used for tls connection during attestz")
		var activeCert string
		if activeCard.MtlsCert != "" {
			activeCert = activeCard.MtlsCert
		} else {
			activeCert = activeCard.ODevIDCert
		}
		wantPeerCert, err := LoadCertificate(activeCert)
		if err != nil {
			t.Fatalf("Error parsing cert. error: %s", err)
		}
		tlsInfo := as.Peer.AuthInfo.(credentials.TLSInfo)
		gotPeerCert := tlsInfo.State.PeerCertificates[0]
		if diff := cmp.Diff(wantPeerCert, gotPeerCert); diff != "" {
			t.Errorf("Incorrect certificate used for attestz tls session. -want,+got:\n%s", diff)
		}
		t.Logf("Verifying attestation for card %v, hash algo: %v", cc.Name, hashAlgo.String())

		cc.verifyAttestation(t, dut, attestResponse, nonce, hashAlgo, PcrIndices)
	}
}

// ParseRoleSelection returns crafted get request with card role.
func ParseRoleSelection(inputRole cdpb.ControlCardRole) *cdpb.ControlCardSelection {
	return &cdpb.ControlCardSelection{
		ControlCardId: &cdpb.ControlCardSelection_Role{
			Role: inputRole,
		},
	}
}

// ParseSerialSelection returns crafted get request with card serial.
func ParseSerialSelection(inputSerial string) *cdpb.ControlCardSelection {
	return &cdpb.ControlCardSelection{
		ControlCardId: &cdpb.ControlCardSelection_Serial{
			Serial: inputSerial,
		},
	}
}

// GetVendorCerts returns vendor certs from the dut for a given card.
func (as *Session) GetVendorCerts(t *testing.T, cardSelection *cdpb.ControlCardSelection) *enrollzpb.GetIakCertResponse {
	enrollzRequest := &enrollzpb.GetIakCertRequest{
		ControlCardSelection: cardSelection,
	}
	t.Logf("Sending Enrollz.GetIakCert request on device: \n %s", PrettyPrint(enrollzRequest))
	response, err := as.EnrollzClient.GetIakCert(context.Background(), enrollzRequest, grpc.Peer(as.Peer))
	if err != nil {
		t.Fatalf("Error getting vendor certs. error: %v", err)
	}
	t.Logf("GetIakCert response: \n %s", PrettyPrint(response))
	return response
}

// RotateOwnerCerts pushes owner certs to the dut for a given card & ssl profile.
func (as *Session) RotateOwnerCerts(t *testing.T, cardRole cdpb.ControlCardRole, oIAKCert string, oDevIDCert string, sslProfileID string) {
	enrollzRequest := &enrollzpb.RotateOIakCertRequest{
		ControlCardSelection: ParseRoleSelection(cardRole),
		OiakCert:             oIAKCert,
		OidevidCert:          oDevIDCert,
		SslProfileId:         sslProfileID,
	}
	t.Logf("Sending Enrollz.Rotate request on device: \n %s", PrettyPrint(enrollzRequest))
	_, err := as.EnrollzClient.RotateOIakCert(context.Background(), enrollzRequest, grpc.Peer(as.Peer))
	if err != nil {
		t.Fatalf("Error with RotateOIakCert. error: %v", err)
	}
	// Brief sleep for rotate to get processed.
	time.Sleep(time.Second)
}

// RequestAttestation requests attestation from the dut for a given card, hash algo & pcr indices.
func (as *Session) RequestAttestation(t *testing.T, cardRole cdpb.ControlCardRole, nonce []byte, hashAlgo attestzpb.Tpm20HashAlgo, pcrIndices []int32) *attestzpb.AttestResponse {
	attestzRequest := &attestzpb.AttestRequest{
		ControlCardSelection: ParseRoleSelection(cardRole),
		Nonce:                nonce,
		HashAlgo:             hashAlgo,
		PcrIndices:           pcrIndices,
	}
	t.Logf("Sending Attestz.Attest request on device: \n %s", PrettyPrint(attestzRequest))
	response, err := as.AttestzClient.Attest(context.Background(), attestzRequest, grpc.Peer(as.Peer))
	if err != nil {
		t.Fatalf("Error with AttestRequest. error: %v", err)
	}
	t.Logf("Attest response: \n %s", PrettyPrint(response))
	return response
}

func (cc *ControlCard) verifyVendorCert(t *testing.T, dut *ondatra.DUTDevice, vendorCaCert []byte, certType string) {
	vendorCa, err := LoadCertificate(string(vendorCaCert))
	if err != nil {
		t.Fatalf("Error loading vendor ca certificate. error: %v", err)
	}

	var cert string
	switch certType {
	case "idevid":
		cert = cc.IDevIDCert
	case "iak":
		cert = cc.IAKCert
	}

	vendorCert, err := LoadCertificate(cert)
	if err != nil {
		t.Fatalf("Error loading vendor certificate. error: %v", err)
	}

	// Formatting time to RFC3339 to ensure ease for comparing.
	expectedTime, _ := time.Parse(time.RFC3339, "9999-12-31T23:59:59Z")
	if !expectedTime.Equal(vendorCert.NotAfter) {
		t.Fatalf("Did not get expected NotAfter date, got: %v, want: %v", vendorCert.NotAfter, expectedTime)
	}

	// Ensure that NotBefore is in the past (should be creation date of the cert, which should always be in the past).
	currentTime := time.Now()
	if currentTime.Before(vendorCert.NotBefore) {
		t.Fatalf("Did not get expected NotBefore date, got: %v, want: earlier than %v", vendorCert.NotBefore, currentTime)
	}

	// Verify cert matches the serial number of the card queried.
	serialNo := gnmi.Get[string](t, dut, gnmi.OC().Component(cc.Name).SerialNo().State())
	if vendorCert.Subject.SerialNumber != serialNo {
		t.Fatalf("Got wrong serial number, got: %v, want: %v", vendorCert.Subject.SerialNumber, serialNo)
	}

	if !strings.EqualFold(vendorCert.Subject.Organization[0], dut.Vendor().String()) {
		t.Fatalf("Wrong signature on Sub Org. got: %v, want: %v", strings.ToLower(vendorCert.Subject.Organization[0]), strings.ToLower(dut.Vendor().String()))
	}

	// Verify cert is signed by switch vendor ca.
	switch vendorCert.SignatureAlgorithm {
	case x509.SHA384WithRSA:
		// Generate Hash from Raw Certificate
		certHash := generateHash(vendorCert.RawTBSCertificate, crypto.SHA384)
		// Retrieve CA Public Key
		vendorCaPubKey := vendorCa.PublicKey.(*rsa.PublicKey)
		// Verify digital signature with oIAK cert.
		err = rsa.VerifyPKCS1v15(vendorCaPubKey, crypto.SHA384, certHash, vendorCert.Signature)
		if err != nil {
			t.Fatalf("Failed verifying vendor cert's signature: %v", err)
		}
	default:
		t.Errorf("Cannot verify signature for %v for cert: %s", vendorCert.SignatureAlgorithm, PrettyPrint(vendorCert))
	}
}

func (cc *ControlCard) verifyControlCardInfo(t *testing.T, dut *ondatra.DUTDevice, gotCardDetails *cdpb.ControlCardVendorId) {
	controller := gnmi.Get[*oc.Component](t, dut, gnmi.OC().Component(cc.Name).State())
	if chassisName == "" {
		chassisName = components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS)[0]
	}
	chassis := gnmi.Get[*oc.Component](t, dut, gnmi.OC().Component(chassisName).State())
	wantCardDetails := &cdpb.ControlCardVendorId{
		ControlCardRole:     cc.Role,
		ControlCardSerial:   controller.GetSerialNo(),
		ControlCardSlot:     string(controller.GetLocation()[len(controller.GetLocation())-1]),
		ChassisManufacturer: chassis.GetMfgName(),
		ChassisSerialNumber: chassis.GetSerialNo(),
		ChassisPartNumber:   chassis.GetPartNo(),
	}

	if diff := cmp.Diff(wantCardDetails, gotCardDetails, protocmp.Transform()); diff != "" {
		t.Errorf("Got diff in vendor card details: -want,+got:\n%s", diff)
	}
}

func generateHash(quote []byte, hashAlgo crypto.Hash) []byte {
	switch hashAlgo {
	case crypto.SHA1:
		quoteHash := sha1.Sum(quote)
		return quoteHash[:]
	case crypto.SHA256:
		quoteHash := sha256.Sum256(quote)
		return quoteHash[:]
	case crypto.SHA384:
		quoteHash := sha512.Sum384(quote)
		return quoteHash[:]
	case crypto.SHA512:
		quoteHash := sha512.Sum512(quote)
		return quoteHash[:]
	}
	return nil
}

// Verify Nokia pcr with expected values. Ensure secure-boot is enabled for pcr values to match.
func nokiaPCRVerify(t *testing.T, dut *ondatra.DUTDevice, cardName string, hashAlgo attestzpb.Tpm20HashAlgo, gotPcrValues map[int32][]byte) error {
	ver := gnmi.Get[string](t, dut, gnmi.OC().System().SoftwareVersion().State())
	t.Logf("Found software version: %v", ver)

	// Expected pcr values for Nokia present in /mnt/nokiaos/<build binary.bin>/known_good_pcr_values.json.
	sshC, err := dut.RawAPIs().BindingDUT().DialCLI(context.Background())
	if err != nil {
		t.Logf("Could not connect ssh. error: %v", err)
	}
	cmd := fmt.Sprintf("cat /mnt/nokiaos/%s/known_good_pcr_values.json", ver)
	res, err := sshC.RunCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Could not run command: %v, error: %v", cmd, err)
	}

	// Parse json file into struct.
	type PcrValuesData struct {
		Pcr   int32  `json:"pcr"`
		Value string `json:"value"`
	}
	type PcrBankData struct {
		Bank   string          `json:"bank"`
		Values []PcrValuesData `json:"values"`
	}
	type CardData struct {
		Card string        `json:"card"`
		Pcrs []PcrBankData `json:"pcrs"`
	}
	type PcrData struct {
		Cards []CardData `json:"cards"`
	}
	var nokiaPcrData PcrData
	err = json.Unmarshal([]byte(res.Output()), &nokiaPcrData)
	if err != nil {
		t.Fatalf("Could not parse json. error: %v", err)
	}

	hashAlgoMap := map[attestzpb.Tpm20HashAlgo]string{
		attestzpb.Tpm20HashAlgo_TPM20HASH_ALGO_SHA1:   "sha1",
		attestzpb.Tpm20HashAlgo_TPM20HASH_ALGO_SHA256: "sha256",
	}

	// Verify pcr_values match expectations.
	pcrIndices := []int32{0, 2, 4, 6, 9, 14}
	cardDesc := gnmi.Get[string](t, dut, gnmi.OC().Component(cardName).Description().State())
	idx := slices.IndexFunc(nokiaPcrData.Cards, func(c CardData) bool {
		return c.Card == cardDesc
	})
	if idx == -1 {
		return fmt.Errorf("could not find card %v in reference data", cardDesc)
	}

	pcrBankData := nokiaPcrData.Cards[idx].Pcrs
	idx = slices.IndexFunc(pcrBankData, func(p PcrBankData) bool {
		return p.Bank == hashAlgoMap[hashAlgo]
	})
	if idx == -1 {
		return fmt.Errorf("could not find pcr bank %v in reference data", hashAlgoMap[hashAlgo])
	}

	wantPcrValues := pcrBankData[idx].Values
	for _, pcrIndex := range pcrIndices {
		idx = slices.IndexFunc(wantPcrValues, func(p PcrValuesData) bool {
			return p.Pcr == pcrIndex
		})
		if idx == -1 {
			return fmt.Errorf("could not find pcr index %v in reference data", pcrIndex)
		}
		if got, want := hex.EncodeToString(gotPcrValues[pcrIndex]), wantPcrValues[idx].Value; got != want {
			t.Errorf("%v pcr %v value does not match expectations, got: %v want: %v", hashAlgoMap[hashAlgo], pcrIndex, got, want)
		}
	}
	return nil
}

func (cc *ControlCard) verifyAttestation(t *testing.T, dut *ondatra.DUTDevice, attestResponse *attestzpb.AttestResponse, wantNonce []byte, pcrHashAlgo attestzpb.Tpm20HashAlgo, pcrIndices []int32) {
	// Verify oIAK cert is the same as the one installed earlier.
	if !cmp.Equal(attestResponse.OiakCert, cc.OIAKCert) {
		t.Errorf("Got incorrect oIAK cert, got: %v, want: %v", attestResponse.OiakCert, cc.OIAKCert)
	}

	// Verify all pcr_values match expectations.
	switch dut.Vendor() {
	case ondatra.NOKIA:
		if err := nokiaPCRVerify(t, dut, cc.Name, pcrHashAlgo, attestResponse.PcrValues); err != nil {
			t.Error(err)
		}
	default:
		t.Error("Vendor reference pcr values not verified.")
	}

	// Retrieve quote signature in TPM Object
	quoteTpmtSignature, err := tpm2.Unmarshal[tpm2.TPMTSignature](attestResponse.QuoteSignature)
	if err != nil {
		t.Fatalf("Error unmarshalling signature. error: %v", err)
	}

	// Default Hash Algo is SHA256 as per TPM2_Quote().
	// https://github.com/tpm2-software/tpm2-tools/blob/master/man/tpm2_quote.1.md
	hashAlgo := crypto.SHA256

	oIakCert, err := LoadCertificate(cc.OIAKCert)
	if err != nil {
		t.Fatalf("Error loading vendor oIAK cert. error: %v", err)
	}

	switch quoteTpmtSignature.SigAlg {
	case tpm2.TPMAlgRSASSA:
		quoteTpmsSignature, err := quoteTpmtSignature.Signature.RSASSA()
		if err != nil {
			t.Fatalf("Error retrieving TPMS signature. error: %v", err)
		}
		// Retrieve signature's hash algorithm.
		hashAlgo, err = quoteTpmsSignature.Hash.Hash()
		if err != nil {
			t.Fatalf("Error retrieving signature hash algorithm. error: %v", err)
		}
		// Generate hash from original quote
		quoteHash := generateHash(attestResponse.Quoted, hashAlgo)
		// Retrieve oIAK public key.
		oIAKPubKey := oIakCert.PublicKey.(*rsa.PublicKey)
		// Verify quote signature with oIAK cert.
		err = rsa.VerifyPKCS1v15(oIAKPubKey, hashAlgo, quoteHash, quoteTpmsSignature.Sig.Buffer)
		if err != nil {
			t.Fatalf("Failed verifying quote signature. error: %v", err)
		}
	default:
		t.Errorf("Cannot verify signature for %v. quote signature: %s", quoteTpmtSignature.SigAlg, PrettyPrint(quoteTpmtSignature))
	}

	// Concatenate pcr values & generate pcr digest.
	var concatPcrs []byte
	for _, idx := range pcrIndices {
		concatPcrs = append(concatPcrs, attestResponse.PcrValues[idx]...)
	}
	wantPcrDigest := generateHash(concatPcrs, hashAlgo)

	// Retrieve pcr digest from quote.
	quoted, err := tpm2.Unmarshal[tpm2.TPMSAttest](attestResponse.Quoted)
	if err != nil {
		t.Fatalf("Error unmarshalling quote. error: %v", err)
	}
	tpmsQuoteInfo, err := quoted.Attested.Quote()
	if err != nil {
		t.Fatalf("Error getting TPMS quote info. error: %v", err)
	}
	gotPcrDigest := tpmsQuoteInfo.PCRDigest.Buffer

	// Verify recomputed PCR digest matches with pcr digest in quote.
	if !cmp.Equal(gotPcrDigest, wantPcrDigest) {
		t.Fatalf("Did not receive expected pcr digest from attest rpc, got: %v, want: %v", gotPcrDigest, wantPcrDigest)
	}

	// Verify nonce.
	gotNonce := quoted.ExtraData.Buffer
	if !cmp.Equal(gotNonce, wantNonce) {
		t.Logf("Did not receive expected nonce, got: %v, want: %v", gotNonce, wantNonce)
	}
}
