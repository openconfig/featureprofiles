/*
 Copyright 2022 Google LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package basetest

import (
	"testing"

	"fmt"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ygot/ygot"
)

var (
	device1  = "dut"
	observer = fptest.NewObserver("System").AddCsvRecorder("ocreport").
			AddCsvRecorder("System")
	systemContainers = []system{
		{
			hostname: ygot.String("tempHost1"),
		},
	}
)

type system struct {
	hostname *string
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func sysGrpcVerify(grpcPort uint16, grpcName string, grpcTs bool, grpcEn bool) {
	if grpcPort == uint16(0) || grpcPort > uint16(0) {
		fmt.Println("Got the expected grpc Port")

	} else {

		errPort := fmt.Errorf("Unexpected value for Port: %v", grpcPort)
		fmt.Println(errPort)
	}
	if grpcName == "DEFAULT" {
		fmt.Println("Got the expected grpc Name")

	} else {
		errName := fmt.Errorf("Unexpected value for Name: %v", grpcName)
		fmt.Println(errName)
	}
	if grpcEn == true {
		fmt.Println("Got the expected grpc Enable")

	} else {
		errEn := fmt.Errorf("Unexpected value for Enable: %v", grpcEn)
		fmt.Println(errEn)
	}
	if grpcTs == false {
		fmt.Println("Got the expected grpc Transport-Security")

	} else {
		errTs := fmt.Errorf("Unexpected value for Transport-Security: %v", grpcTs)
		fmt.Println(errTs)
	}

}
