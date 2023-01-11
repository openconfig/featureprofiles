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

4.  Tests should make use of Ondatra Raw API `dut.RawAPIs().P4RT().Default(t)`
    during client instantiation.

5.  To avoid indefinite blocking during Get response
    `client.StreamChannelGetArbitrationResp()`, tests should make use of
    `p4rtutils.StreamTermErr()` function which returns an Error if the p4rt
    stream has been terminated.

    *   The test blocks indefinitely because
        `client.StreamChannelGetArbitrationResp()` internally calls
        `stream.GetArbitration(minSeqNum)`. If `minSeqNum` is non zero, the
        process blocks (using `sync.Cond.Wait()`) until atleast `minSeqNum`
        number of Arbitration responses are received. This method doesn't take
        into account a scenario where the device terminates the `stream` and
        would never send an Arbitration response.

    ```go
    func (p *P4RTClientStream) GetArbitration(minSeqNum uint64) (uint64, *P4RTArbInfo) {
        p.arb_mu.Lock()
        defer p.arb_mu.Unlock()

        for p.arbCounters.RxArbCntr < minSeqNum {
            if glog.V(2) {
                glog.Infof("'%s' Waiting on Arbitration message (%d/%d)\n",
                    p, p.arbCounters.RxArbCntr, minSeqNum)
            }
            p.arbCond.Wait() // Blocks indefinitely here if (p *P4RTClientStream) has been terminated by the device.
        }

        if len(p.arbQ) == 0 {
            return p.arbCounters.RxArbCntr, nil
        }

        arbInfo := p.arbQ[0]
        p.arbQ = p.arbQ[1:]

        return p.arbCounters.RxArbCntr, arbInfo
    }
    ```

    *   As a workaround, we need to call `p4rtutils.StreamTermErr()` after
        `StreamChannelCreate()` and before `StreamChannelGetArbitrationResp()`.
        This workaround however doesn't fix the underlying issue with the
        client. A proper fix would be to replace the `Cond.Wait()` with a
        timeout.

    ```go
    client.StreamChannelCreate(&streamParameter)
    if err := client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
        Update: &p4_v1.StreamMessageRequest_Arbitration{
            Arbitration: &p4_v1.MasterArbitrationUpdate{
                DeviceId: streamParameter.DeviceId,
                ElectionId: &p4_v1.Uint128{
                    High: streamParameter.ElectionIdH,
                    Low:  streamParameter.ElectionIdL,
                },
            },
        },
    }); err != nil {
        return errors.New("Errors seen when sending ClientArbitration message.")
    }
    if err := p4rtutils.StreamTermErr(client.StreamTermErr); err != nil {
        return err
    }
    if _, _, arbErr := client.StreamChannelGetArbitrationResp(&streamName, 1); arbErr != nil {
        return errors.New("Errors seen in ClientArbitration response.")
    }
    ```

6.  Tests should get the P4RT Node Name by walking the Components OC tree.
    Components of type `INTEGRATED_CIRCUIT` should have child Components of type
    `PORT`. These PORT Components can be mapped to currently reserved Interfaces
    using the `hardware-port` leaf in the Interfaces tree. Such an
    implementation already exists in `p4rtutils` library:
    `p4rtutils.P4RTNodesByPort()`.

7.  If P4RT Node Names cannot be resolved by walking the Components tree, use
    deviation flag `--deviation_explicit_p4rt_node_component` and pass the node
    names through args `--arg_p4rt_node_name_1`, `--arg_p4rt_node_name_2`.
