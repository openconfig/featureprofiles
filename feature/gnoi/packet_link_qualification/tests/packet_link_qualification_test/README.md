# gNOI-2.1: Packet-based Link Qualification

## Summary

Validate gNOI RPC can support packet-based link qualification test for various port speeds and configurations within 1 DUT.
This includes:
*   Homogeneous 100G and 400G links (singleton and memberlink).
*   Heterogeneous breakout and overlaid Port-Channel configurations (e.g., native 800G/400G to breakout 8x100G/4x100G).

## Topology

### Homogeneous Topology
*   dut1:port 1 <--> port 2:dut1 - 400G ports (port 1 and 2 as singleton and memberlink)
*   dut1:port 3 <--> port 4:dut1 - 100G ports (port 3 and 4 as singleton and memberlink)

### Heterogeneous & Overlaid Port-Channel Topology (b/449074843)
*   dut1:port 1 (Generator, native speed e.g., 400G) <--> port 2:dut1 (Reflector, breakout sub-port e.g., 4x100G channel)
*   Both ports are members of a Port-Channel (LAG) in the device's base configuration.

## Procedure

### General Validations (Applies to all cases)
*   Validate the link qualification Capabilities response.
    *   MaxHistoricalResultsPerInterface is >= 2.
    *   Time exists.
    *   Generator:
        *   MinMtu >= 64,
        *   MaxMtu >= 8184,
        *   MaxBps >= 4e11,
        *   MaxPps >= 5e8,
        *   MinSetupDuration > 0
        *   MinTeardownDuration > 0,
        *   MinSampleInterval > 0,
    *   Reflector:
        *   MinSetupDuration > 0
        *   MinTeardownDuration > 0,
*   Validate the error code is returned for Get and Delete requests with non-existing ID.
    *   Error code is 5 NOT_FOUND (HTTP Mapping: 404 Not Found).
*   Validate the link qualification List and Delete.
    *   Issue List qualifications request.
    *   Delete the qualification if qualification is found.
    *   Issue List qualifications request again.
    *   Verify that the qualification has been deleted successfully by checking List response.

### Test Case 1: Homogeneous Link Qualification (100G/400G)
*   Set a port as the NEAR_END (generator) device for Packet Based Link Qual.
    *   Issue gnoi.LinkQualification Create RPC to the device and provide following parameters:
        *   Id: A unique identifier for this run of the test
        *   InterfaceName: interface as the interface to be used as generator end.
            *   This interface must be connected to the interface chosen on the reflector device.
        *   EndpointType: Qualification_end set as NEAR_END with PacketGeneratorConfiguration.
    *   Set the following parameters for link qualification service usage:
        *   PacketRate: Packet per second rate to use for this test.
        *   PacketSize: Size of packets to inject. The value is 8184 bytes.
    *   RPCSyncedTiming:
        *   SetupDuration: The requested setup time for the endpoint.
        *   PreSyncDuration: Minimum_wait_before_preparation_seconds. Within this period, the device should:
            *   Initialize the link qualification state machine.
            *   Set port’s state to TESTING. This state is only relevant inside the linkQual service.
            *   Set the port in loopback mode.
        *   Duration:The length of the qualification.
        *   PostSyncDuration: The amount time a side should wait before starting its teardown.
        *   TeardownDuration: The amount time required to bring the interface back to pre-test state.
    *   Verify generator interface oper-state is 'TESTING'
*   Set another port as the FAR_END (reflector) device for Packet Based Link Qual.
    *   Issue gnoi.LinkQualification Create RPC to the device and provide following parameters:
        *   Id: A unique identifier for this run of the test
        *   InterfaceName: Interface as the interface to be used as a reflector to turn the packet back.
        *   EndpointType: Qualification_end set as FAR_END (ASIC or PMD loopback as supported).
        *   RPCSyncedTiming:
            *   Reflector timers should be same as the ones on the generator.
        *   Verify reflector interface oper-state is 'TESTING'
*   Get the result by issuing gnoi.LinkQualification Get RPC to gather the result of link qualification. Provide the following parameter:
    *   Id: The identifier used above on the NEAR_END side.
    *   Validate the response to:
        *   Ensure that the current_state is QUALIFICATION_STATE_COMPLETED
        *   Ensure that the num_corrupt_packets and num_packets_dropped_by_mmu are 0
        *   Ensure that RPC status code is 0 for success.
        *   Packets sent count matches with packets received.

### Test Case 2: Heterogeneous Breakout & Overlaid Port-Channel Link Qualification (b/449074843)
*   **Configure Heterogeneous Topology**:
    *   Setup the physical/logical links such that the Generator port on the local DUT is in native speed (e.g., 400G), and the Reflector port on the peer DUT is a breakout sub-port (e.g., 4x100G channel).
*   **Apply Production Profile (Overlaid LAG)**:
    *   Map both interfaces as members of a Port-Channel (LAG) in the device's base configuration.
*   **Trigger Reflector (Far-End)**:
    *   Issue the `gnoi.LinkQualification.Create` RPC to the far-end DUT, setting its `EndpointType` as `REFLECTOR` (ASIC or PMD loopback as supported).
*   **Trigger Generator (Near-End)**:
    *   Issue the `gnoi.LinkQualification.Create` RPC to the local DUT, setting its `EndpointType` as `NEAR_END` (PacketGenerator) and defining the target packet size (e.g., max MTU) and transmission rate.
*   **Verify State Transition**:
    *   Verify that the RPC does not return an error and both state machines transition to `QUALIFICATION_STATE_COMPLETED`.
*   **Assert Results**:
    *   Query the results using `gnoi.LinkQualification.Get` and assert that the packet loss ratio L = 0 (or within minor acceptable hardware tolerance).
    *   Verify that the reflector successfully reflects packets despite the overlaid LAG configuration.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "LAG - Member - port1",
          "enabled": true,
          "name": "port1",
          "type": "ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "lag3"
          }
        },
        "name": "port1"
      },
      {
        "aggregation": {
          "config": {
            "lag-type": "STATIC"
          }
        },
        "config": {
          "description": "lag3",
          "enabled": true,
          "mtu": 9000,
          "name": "lag3",
          "type": "ieee8023adLag"
        },
        "name": "lag3",
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.1",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC paths used for test setup are not listed here.

```yaml
paths:
  /interfaces/interface/state/oper-status:
rpcs:
  gnoi:
    packet_link_qualification.LinkQualification.Capabilities:
    packet_link_qualification.LinkQualification.Create:
    packet_link_qualification.LinkQualification.Delete:
    packet_link_qualification.LinkQualification.Get:
    packet_link_qualification.LinkQualification.List:
```
## Required DUT platform

* Specify the minimum DUT-type
    * FFF - fixed form factor is enough for this test. However it can run also
      on a MFF testbed.
      gNMI.Set:

## Minimum DUT platform requirement
* FFF - fixed form factor

