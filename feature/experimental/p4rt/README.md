# P4RT Implementation Guide

This document specifies the requirements for p4rt test implementation.

1.  Use the [cisco-open/go-p4 library](https://github.com/cisco-open/go-p4).

2.  The client should import or make use of the following WBB information in the
    following Google compatible format:

    1.  WBB P4 Protobuf file:
        https://github.com/openconfig/featureprofiles/blob/main/feature/experimental/p4rt/wbb.p4info.pb.txt

3.  Tests should create new P4RT clients using the `p4rt_client.NewP4RTClient()`
    function, which sets up the `StreamTermErr` channel required to check errors
    when the p4rt stream terminates.

4.  Tests should make use of Ondatra Raw API `dut.RawAPIs().P4RT(t)`
    during client instantiation.

5.  Tests should log Stream Termination errors populated in the
    `client.StreamTermErr` channel using the `p4rtutils.StreamTermErr()` helper
    if there are errors in GetArbitration response or GetPacket response.

    ```go
    client.StreamChannelCreate(&streamParameter)
    if err := client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
        Update: &p4_v1.StreamMessageRequest_Arbitration{
            Arbitration: &p4_v1.MasterArbitrationUpdate{
                DeviceId: streamParameter.DeviceId,
                ElectionId: &p4_v1.Uint128{
                    High: streamParameter.ElectionIdH,
                    Low:  streamParameter.ElectionIdL - uint64(index),
                },
            },
        },
    }); err != nil {
        return fmt.Errorf("errors seen when sending ClientArbitration message: %v", err)
    }
    if _, _, arbErr := client.StreamChannelGetArbitrationResp(&streamName, 1); arbErr != nil {
        if err := p4rtutils.StreamTermErr(client.StreamTermErr); err != nil {
            return err
        }
        return fmt.Errorf("errors seen in ClientArbitration response: %v", arbErr)
            }
    ```

6.  Tests should get the P4RT Node Name by walking the Components OC tree.
    Components of type `INTEGRATED_CIRCUIT` should have child Components of type
    `PORT`. These PORT Components can be mapped to currently reserved Interfaces
    using the `hardware-port` leaf in the Interfaces tree. Such an
    implementation already exists in `p4rtutils` library:
    `p4rtutils.P4RTNodesByPort()`.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml

```
