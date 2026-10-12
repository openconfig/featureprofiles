// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package macsec_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"google.golang.org/grpc/metadata"

	"github.com/openconfig/functional-translators/registrar"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
)

const (
	ip1      = "10.0.0.1/30"
	ip2      = "10.0.0.2/30"
	username = "macsec_test_user"
	password = "macsec_test_password"
)

func generateHexKey(length int) string {
	b := make([]byte, length/2)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

var (
	keyID     = generateHexKey(64)
	secretKey = generateHexKey(64)
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureTestUser(t *testing.T, dut *ondatra.DUTDevice) {
	if dut.Vendor() == ondatra.ARISTA {
		cli := fmt.Sprintf(`
username %s privilege 15 role network-admin secret 0 %s
!
`, username, password)
		dut.Config().New().WithText(cli).Push(t)
		t.Logf("Configured test user on DUT %s", dut.Name())
	}
}

func configureMacsecOnDUT(t *testing.T, dut *ondatra.DUTDevice, port *ondatra.Port, ipv4, customKeyID, customSecretKey string) {
	if dut.Vendor() == ondatra.ARISTA {
		cli := fmt.Sprintf(`
mac security
   profile must_secure
      cipher aes256-gcm
      key %s 0 %s
!
interface %s
   no switchport
   ip address %s
   mac security profile must_secure
`, customKeyID, customSecretKey, port.Name(), ipv4)
		dut.Config().New().WithText(cli).Append(t)
		t.Logf("Configured MACsec on DUT %s, port %s", dut.Name(), port.Name())
	}
}

func getTranslatedUpdates(t *testing.T, dut *ondatra.DUTDevice, ftName string) []*gpb.Update {
	t.Helper()
	ctx := metadata.AppendToOutgoingContext(context.Background(),
		"username", username,
		"password", password,
	)
	gnmiClient := dut.RawAPIs().GNMI(t)

	ft, ok := registrar.FunctionalTranslatorRegistry[ftName]
	if !ok {
		t.Fatalf("Functional translator %q not found.", ftName)
	}

	var nativePaths []*gpb.Path
	for _, paths := range ft.OutputToInputMap() {
		nativePaths = append(nativePaths, paths...)
	}

	if len(nativePaths) == 0 {
		t.Fatalf("No native paths found for functional translator %q", ftName)
	}

	resp, err := gnmiClient.Get(ctx, &gpb.GetRequest{
		Path:     nativePaths,
		Type:     gpb.GetRequest_STATE,
		Encoding: gpb.Encoding_JSON_IETF,
	})
	if err != nil {
		t.Fatalf("[%s] Failed to get native paths: %v", dut.Name(), err)
	}

	var updates []*gpb.Update
	for _, notification := range resp.GetNotification() {
		dummySR := &gpb.SubscribeResponse{
			Response: &gpb.SubscribeResponse_Update{
				Update: notification,
			},
		}
		translatedSR, err := ft.Translate(dummySR)
		if err != nil {
			t.Logf("[%s] Translation Failed: %v", dut.Name(), err)
			continue
		}
		if translatedSR == nil {
			continue
		}
		updates = append(updates, translatedSR.GetUpdate().GetUpdate()...)
	}
	return updates
}

func verifyNoStatusOrCkn(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	if dut.Vendor() != ondatra.ARISTA {
		return
	}
	for _, update := range getTranslatedUpdates(t, dut, deviations.MacsecStateFt(dut)) {
		path := update.GetPath()
		pathStr := ""
		for _, elem := range path.GetElem() {
			pathStr += "/" + elem.GetName()
		}
		if strings.Contains(pathStr, "status") || strings.Contains(pathStr, "ckn") {
			t.Fatalf("[%s] Found unexpected status or CKN element before MACsec configured: %s = %v", dut.Name(), pathStr, update.GetVal())
		}
	}
}

func verifyStatusAndCkn(t *testing.T, dut *ondatra.DUTDevice, expectedCkn string) {
	t.Helper()
	if dut.Vendor() != ondatra.ARISTA {
		return
	}

	var finalCkn string
	success := false
	for i := 0; i < 10; i++ {
		statusSecured := false
		for _, update := range getTranslatedUpdates(t, dut, deviations.MacsecStateFt(dut)) {
			path := update.GetPath()
			pathStr := ""
			for _, elem := range path.GetElem() {
				pathStr += "/" + elem.GetName()
			}

			if strings.Contains(pathStr, "status") {
				var valStr string
				if list := update.GetVal().GetLeaflistVal(); list != nil && len(list.GetElement()) > 0 {
					valStr = list.GetElement()[0].GetStringVal()
				} else {
					valStr = update.GetVal().GetStringVal()
				}
				t.Logf("[%s | loop %d] Status: %s = %s", dut.Name(), i+1, pathStr, valStr)
				if valStr == "Secured" {
					statusSecured = true
				}
			}
			if strings.Contains(pathStr, "ckn") {
				var valStr string
				if list := update.GetVal().GetLeaflistVal(); list != nil && len(list.GetElement()) > 0 {
					valStr = list.GetElement()[0].GetStringVal()
				} else {
					valStr = update.GetVal().GetStringVal()
				}
				finalCkn = valStr
			}
		}
		if statusSecured {
			success = true
			break
		}
		time.Sleep(6 * time.Second)
	}

	if !success {
		t.Fatalf("[%s] Failed to verify status reached Secured state", dut.Name())
	}

	if finalCkn != expectedCkn {
		t.Fatalf("[%s] CKN mismatch: got %q, want %q", dut.Name(), finalCkn, expectedCkn)
	} else {
		t.Logf("[%s] Successfully verified status=Secured and exactly matched CKN=%q", dut.Name(), expectedCkn)
	}
}

func getMacsecCounter(t *testing.T, dut *ondatra.DUTDevice, port string, counterName string) uint64 {
	t.Helper()
	if dut.Vendor() != ondatra.ARISTA {
		return 0
	}
	for _, update := range getTranslatedUpdates(t, dut, deviations.MacsecCountersFt(dut)) {
		path := update.GetPath()
		foundInterface := false
		for _, elem := range path.GetElem() {
			if elem.GetName() == "interface" && elem.GetKey()["name"] == port {
				foundInterface = true
				break
			}
		}
		if !foundInterface {
			continue
		}
		pathStr := ""
		for _, elem := range path.GetElem() {
			pathStr += "/" + elem.GetName()
		}
		if strings.Contains(pathStr, counterName) {
			return update.GetVal().GetUintVal()
		}
	}
	t.Errorf("[%s] Counter %s on port %s not found", dut.Name(), counterName, port)
	return 0
}

func verifyCounterIncrements(t *testing.T, dut *ondatra.DUTDevice, port string, counterName string) {
	t.Helper()
	if dut.Vendor() != ondatra.ARISTA {
		return
	}
	startVal := getMacsecCounter(t, dut, port, counterName)
	t.Logf("[%s] Initial value for %s on port %s: %d", dut.Name(), counterName, port, startVal)

	success := false
	for i := 0; i < 10; i++ {
		time.Sleep(6 * time.Second)
		currentVal := getMacsecCounter(t, dut, port, counterName)
		t.Logf("[%s | loop %d] %s on port %s: %d", dut.Name(), i+1, counterName, port, currentVal)
		if currentVal > startVal {
			success = true
			break
		}
	}
	if !success {
		t.Errorf("[%s] Counter %s on port %s did not increment from %d over 60 seconds", dut.Name(), counterName, port, startVal)
	}
}

func verifyCounterNonZero(t *testing.T, dut *ondatra.DUTDevice, port string, counterName string) {
	t.Helper()
	if dut.Vendor() != ondatra.ARISTA {
		return
	}
	success := false
	for i := 0; i < 5; i++ {
		val := getMacsecCounter(t, dut, port, counterName)
		if val > 0 {
			success = true
			t.Logf("[%s] Verified %s on port %s is non-zero: %d", dut.Name(), counterName, port, val)
			break
		}
		time.Sleep(2 * time.Second)
	}
	if !success {
		t.Fatalf("[%s] Counter %s on port %s is zero", dut.Name(), counterName, port)
	}
}

func sendGnoiPing(t *testing.T, dut *ondatra.DUTDevice, destination string) {
	t.Helper()
	gnoiClient := dut.RawAPIs().GNOI(t)
	req := &spb.PingRequest{
		Destination: destination,
		Count:       15,
	}
	pingClient, err := gnoiClient.System().Ping(context.Background(), req)
	if err != nil {
		t.Logf("[%s] Failed to send gnoi ping (expected if encryption broken): %v", dut.Name(), err)
		return
	}
	for {
		_, err := pingClient.Recv()
		if err != nil {
			break
		}
	}
}

func TestMacsecConfiguration(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	port1 := dut1.Port(t, "port1")
	port2 := dut2.Port(t, "port1")

	configureTestUser(t, dut1)
	configureTestUser(t, dut2)

	// Adding buffer to ensure credentials are synced!
	time.Sleep(10 * time.Second)

	t.Run("VerifyStatusAndCknReported", func(t *testing.T) {
		verifyNoStatusOrCkn(t, dut1)
		verifyNoStatusOrCkn(t, dut2)

		configureMacsecOnDUT(t, dut1, port1, ip1, keyID, secretKey)
		configureMacsecOnDUT(t, dut2, port2, ip2, keyID, secretKey)

		verifyStatusAndCkn(t, dut1, keyID)
		verifyStatusAndCkn(t, dut2, keyID)

		verifyCounterNonZero(t, dut1, port1.Name(), "tx-pkts-ctrl")
		verifyCounterNonZero(t, dut1, port1.Name(), "rx-pkts-ctrl")
		verifyCounterNonZero(t, dut2, port2.Name(), "tx-pkts-ctrl")
		verifyCounterNonZero(t, dut2, port2.Name(), "rx-pkts-ctrl")
	})

	t.Run("RxUnrecognizedCkn", func(t *testing.T) {
		keyID1 := generateHexKey(64)
		keyID2 := generateHexKey(64)

		configureMacsecOnDUT(t, dut1, port1, ip1, keyID1, secretKey)
		configureMacsecOnDUT(t, dut2, port2, ip2, keyID2, secretKey)

		verifyCounterIncrements(t, dut2, port2.Name(), "rx-unrecognized-ckn")
	})

	t.Run("RxBadIcvPkts", func(t *testing.T) {
		secretKey1 := generateHexKey(64)
		secretKey2 := generateHexKey(64)

		configureMacsecOnDUT(t, dut1, port1, ip1, keyID, secretKey1)
		configureMacsecOnDUT(t, dut2, port2, ip2, keyID, secretKey2)

		// Push gNOI pings to guarantee a robust volume of encrypted (bad ICV) packets across the L3 channel
		sendGnoiPing(t, dut1, "10.0.0.2")

		verifyCounterIncrements(t, dut2, port2.Name(), "rx-badicv-pkts")
	})

	t.Run("VerifyHardwareExceptionCountersReported", func(t *testing.T) {
		configureMacsecOnDUT(t, dut1, port1, ip1, keyID, secretKey)
		configureMacsecOnDUT(t, dut2, port2, ip2, keyID, secretKey)

		for _, counterName := range []string{"tx-pkts-err-in", "tx-pkts-dropped", "rx-pkts-dropped"} {
			val1 := getMacsecCounter(t, dut1, port1.Name(), counterName)
			val2 := getMacsecCounter(t, dut2, port2.Name(), counterName)
			t.Logf("[%s] Counter %s on port %s = %d", dut1.Name(), counterName, port1.Name(), val1)
			t.Logf("[%s] Counter %s on port %s = %d", dut2.Name(), counterName, port2.Name(), val2)
		}
	})
}
