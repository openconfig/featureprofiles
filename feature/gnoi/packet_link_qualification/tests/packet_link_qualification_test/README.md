# gNOI-2.1: Packet-based Link Qualification

## Summary

Validate gNOI RPC can support packet-based link qualification test for the links
between 2 DUTs.

## Topology

*   dut1:port1 <--> port1:dut2

## Procedure

*   Connect two vendor devices back to back on 100G bandwidth ports.
    *   Validate the link qualification Capabilities response.
        *   MaxHistoricalResultsPerInterface is >= 2.
        *   Time exists.
        *   Generator:
            *   MinMtu >= 64,
            *   MaxMtu >= 9000,
            *   MaxBps >= 4e11,
            *   MaxPps >= 5e8,
            *   MinSetupDuration > 0
            *   MinTeardownDuration > 0,
            *   MinSampleInterval > 0,
        *   Reflector:
            *   MinSetupDuration > 0
            *   MinTeardownDuration > 0,
*   Validate the error code is returned for Get and Delete requests with
    non-existing ID.
    *   Error code is 5 NOT_FOUND (HTTP Mapping: 404 Not Found).
*   Validate the link qualification List and Delete.
    *   Issue List qualifications request.
    *   Delete the qualification if qualification is found.
    *   Issue List qualifications request again.
    *   Verify that the qualification has been deleted successfully by checking
        List response.
*   Set a device as the NEAR_END (generator) device for Packet Based Link Qual.
    *   Issue gnoi.LinkQualification Create RPC to the device and provide
        following parameters:
        *   Id: A unique identifier for this run of the test
        *   InterfaceName: interface as the interface to be used as generator
            end.
            *   This interface must be connected to the interface chosen on the
                reflector device using 100G connection.
        *   EndpointType: Qualification_end set as NEAR_END with
            PacketGeneratorConfiguration.
    *   Set the following parameters for link qualification service usage:
        *   PacketRate: Packet per second rate to use for this test.
        *   PacketSize: Size of packets to inject. The value is 9000 bytes.
    *   RPCSyncedTiming:
        *   SetupDuration: The requested setup time for the endpoint.
        *   PreSyncDuration: Minimum_wait_before_preparation_seconds. Within
            this period, the device should:
            *   Initialize the link qualification state machine.
            *   Set port’s state to TESTING. This state is only relevant inside
                the linkQual service.
            *   Set the port in loopback mode.
        *   Duration:The length of the qualification.
        *   PostSyncDuration: The amount time a side should wait before starting
            its teardown.
        *   TeardownDuration: The amount time required to bring the interface
            back to pre-test state.
*   Set another device as the FAR_END (reflector) device for Packet Based Link
    Qual.
    *   Issue gnoi.LinkQualification Create RPC to the device and provide
        following parameters:
        *   Id: A unique identifier for this run of the test
        *   InterfaceName: Interface as the interface to be used as a reflector
            to turn the packet back.
        *   EndpointType: Qualification_end set as FAR_END.
        *   RPCSyncedTiming:
            *   Reflector timers should be same as the ones on the generator.
*   Get the result by issuing gnoi.LinkQualification Get RPC to gather the
    result of link qualification. Provide the following parameter:
    *   Id: The identifier used above on the NEAR_END side.
    *   Validate the response to
        *   Ensure that the current_state is QUALIFICATION_STATE_COMPLETED
        *   Ensure that the num_corrupt_packets and num_packets_dropped_by_mmu
            are 0
        *   Ensure that RPC status code is 0 for succuss.
        *   GetQualificationRateBytesPerSecond matches
            GetExpectedRateBytesPerSecond in the result.

## Telemetry Parameter Coverage

*   None
