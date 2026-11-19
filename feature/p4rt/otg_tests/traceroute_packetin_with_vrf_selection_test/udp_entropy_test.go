// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package traceroute_packetin_with_vrf_selection_test

import (
	"fmt"
	"math/rand"

	"github.com/open-traffic-generator/snappi/gosnappi"
)

const (
	traceroutePacketsPerFlow = 1000
	tracerouteTrafficRatePPS = 50
	distributionFlowCount    = 4
)

// distributionFlowArgs gives Test-1 independent, reproducible hash samples.
// Split the original 50 pps across the samples so the larger PacketIn sample
// does not increase the offered packet rate.
func distributionFlowArgs(base flowArgs) []*flowArgs {
	samples := make([]*flowArgs, 0, distributionFlowCount)
	for i := 0; i < distributionFlowCount; i++ {
		sample := base
		sample.flowName = fmt.Sprintf("%s-sample-%d", base.flowName, i+1)
		sample.udpEntropySeedOffset = int64(i * 4)
		sample.ratePps = uint64(tracerouteTrafficRatePPS / distributionFlowCount)
		if i < tracerouteTrafficRatePPS%distributionFlowCount {
			sample.ratePps++
		}
		samples = append(samples, &sample)
	}
	return samples
}

// configureUDPFlowEntropy spreads the fixed packet burst over distinct hash
// inputs. Separate seeds for source/destination and outer/inner headers avoid
// the identical port counters previously used in all four fields. Fixed seeds
// make a hardware distribution failure reproducible across reruns.
func configureUDPFlowEntropy(udp gosnappi.FlowUdp, srcSeed, dstSeed int64) {
	udp.SrcPort().SetValues(udpPortValues(srcSeed))
	udp.DstPort().SetValues(udpPortValues(dstSeed))
}

func udpPortValues(seed int64) []uint32 {
	const firstPort, lastPort = 1024, 65535
	ports := rand.New(rand.NewSource(seed)).Perm(lastPort - firstPort + 1)
	values := make([]uint32, traceroutePacketsPerFlow)
	for i := range values {
		values[i] = uint32(ports[i] + firstPort)
	}
	return values
}
