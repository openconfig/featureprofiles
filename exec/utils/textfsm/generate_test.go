package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	textfsm "github.com/openconfig/featureprofiles/exec/utils/textfsm/textfsm"
)

func TestRegisterPluginAndGetPingEthernetStruct(t *testing.T) {
	tests := []struct {
		name  string
		input string
		out   string
		want  textfsm.GoTextFSMStruct
	}{{
		name:  "Test register plugin and get ping ethernet struct",
		input: "ping",
		out: `Type escape sequence to abort.
		Sending 5 CFM Loopbacks, timeout is 1 seconds -
		Domain d1 (level 5), Service s1
		Source: MEP ID 11, interface GigabitEthernet0/2/0/20.2
		Target: 001d.e5eb.890b (MEP ID 1):
		  Running (5s) ...
		Success rate is 100.0 percent (5/5), round-trip min/avg/max = 1/0/1 ms
		Out-of-sequence: 20.0 percent (1/5)
		Bad data: 0.0 percent (0/5)
		Received packet rate: 1.7 pps
		`,
		want: &textfsm.PingEthernet{
			Rows: []textfsm.PingEthernetRow{
				{
					RoundTripAvg: "0",
					RoundTripMax: "1",
					RoundTripMin: "1",
					RxCount:      "5",
					SuccessRate:  "100.0",
					TxCount:      "5",
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pingEthernet := &textfsm.PingEthernet{}
			if err := pingEthernet.Parse(test.out); err != nil {
				t.Fatalf("%v", err)
			}

			t.Logf("%+v\n", pingEthernet)

			if !cmp.Equal(pingEthernet, test.want) {
				t.Fatalf("got %v, want %v", pingEthernet, test.want)
			}
		})
	}
}
