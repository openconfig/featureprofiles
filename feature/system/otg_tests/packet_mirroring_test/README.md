# DP-1.18: Flow matching using ACL and to Port Mirror/Redirect

## Overview
This test case verifies Google-specific packet mirroring functionality on the
device, redirecting traffic matching an ACL to a local destination file. It validates
four distinct mirror attachment scenarios:
1. Ingress Physical Interface
2. Egress Physical Interface
3. Ingress LAG member port
4. Egress LAG member port

It utilizes the `/mirroring` top-level OpenConfig tree defined in the `google-packet-mirror` YANG extension.

## Testbed Topology
A standard `TESTBED_DUT_ATE_4LINKS` setup is used:
- Two links configured as a Link Aggregation Group (LAG/Port-Channel) between the DUT and ATE.
- Two links configured as standalone interfaces between the DUT and ATE.

## Procedure

### Configuration
1. **LAG Configuration**:
   - Configure a static or dynamic LAG (e.g., `lag1`) on the DUT and ATE with two member links (Port A and Port B).
   - Configure IPv4 and IPv6 addresses on both standalone and LAG interfaces.
2. **ACL Configuration**:
   - Configure an ACL set (e.g. `mirror-acl`) on the DUT matching specific test traffic (e.g., matching a particular source IP / port).
3. **Packet Mirroring Configuration**:
   - Create a mirror session (e.g. `session-ingress`) using the `/mirroring/sessions/session` path.
   - Set `mirror-action` to `FILE` to redirect to a local file.
   - Configure a maximum capture size (e.g., `max-capture-size = 1000000` bytes).
   - Initially set `enabled = false`.

### Test Cases

#### Test Case 1: Ingress Physical Interface Mirroring
1. **Config**:
   - Associate the mirror session with a standalone ingress physical interface (Port C) on the DUT.
   - Set `direction = RX` and bind the `mirror-acl` to the session.
2. **Enable**:
   - Set the session `enabled = true`.
3. **Traffic Injection**:
   - Instruct the ATE to transmit traffic matching the ACL to Port C on the DUT.
   - Instruct the ATE to transmit traffic *not* matching the ACL to Port C.
4. **Disable**:
   - Set the session `enabled = false`.
5. **Verification**:
   - Query `/mirroring/sessions/session[name=session-ingress]/state` to ensure the session status is updated correctly.
   - Retrieve the mirrored pcap file from the DUT using the gNOI File.Get RPC.
   - Assert that:
     - The file exists on the DUT and is successfully retrieved.
     - The file size is greater than 0.
     - (Optional) Read the pcap contents and verify that only packets matching the ACL are captured, and no non-matching packets are captured.

#### Test Case 2: Egress Physical Interface Mirroring
1. **Config**:
   - Associate the mirror session with a standalone egress physical interface (Port D) on the DUT.
   - Set `direction = TX` and bind the `mirror-acl` to the session.
2. **Enable**:
   - Set the session `enabled = true`.
3. **Traffic Injection**:
   - Instruct the ATE to transmit traffic matching the ACL such that it egresses Port D on the DUT.
   - Transmit non-matching traffic.
4. **Disable**:
   - Set the session `enabled = false`.
5. **Verification**:
   - Retrieve the mirrored file via gNOI File.Get and verify its existence and size.
   - Assert that egress traffic matching the ACL was captured.

#### Test Case 3: LAG Member Port Ingress Mirroring
1. **Config**:
   - Associate the mirror session with one of the physical member interfaces of the LAG (Port A).
   - Set `direction = RX` and bind the `mirror-acl` to the session.
2. **Enable**:
   - Set the session `enabled = true`.
3. **Traffic Injection**:
   - Instruct the ATE to transmit hashing-distributed traffic (such that a portion ingress Port A) matching the ACL.
4. **Disable**:
   - Set the session `enabled = false`.
5. **Verification**:
   - Retrieve the mirrored file via gNOI File.Get.
   - Verify that packets entering the LAG member Port A that match the ACL are successfully captured in the local file.

#### Test Case 4: LAG Member Port Egress Mirroring
1. **Config**:
   - Associate the mirror session with one of the physical member interfaces of the LAG (Port B).
   - Set `direction = TX` and bind the `mirror-acl` to the session.
2. **Enable**:
   - Set the session `enabled = true`.
3. **Traffic Injection**:
   - Transmit matching traffic over the LAG from the DUT to the ATE such that packets egress Port B.
4. **Disable**:
   - Set the session `enabled = false`.
5. **Verification**:
   - Retrieve the mirrored file via gNOI File.Get.
   - Verify that packets egressing the LAG member Port B that match the ACL are captured.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  /interfaces/interface/config/name:
  /interfaces/interface/config/enabled:
  /interfaces/interface/ethernet/config/aggregate-id:
  /acl/acl-sets/acl-set/config/name:
  /acl/acl-sets/acl-set/config/type:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/config/sequence-id:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/actions/config/forwarding-action:
  /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/set-name:
  /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/type:
rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
  gnoi:
    gnoi.file.File.Get:
    gnoi.file.File.Remove:
```

## Canonical OC
```json
{
  "interfaces": {
    "interface": [
      {
        "name": "portc",
        "config": {
          "name": "portc",
          "enabled": true
        }
      },
      {
        "name": "lag1",
        "config": {
          "name": "lag1",
          "enabled": true,
          "type": "ieee8023adLag"
        }
      }
    ]
  },
  "acl": {
    "acl-sets": {
      "acl-set": [
        {
          "name": "mirror-acl",
          "type": "ACL_IPV4",
          "config": {
            "name": "mirror-acl",
            "type": "ACL_IPV4"
          },
          "acl-entries": {
            "acl-entry": [
              {
                "sequence-id": 10,
                "config": {
                  "sequence-id": 10
                },
                "actions": {
                  "config": {
                    "forwarding-action": "ACCEPT"
                  }
                }
              }
            ]
          }
        }
      ]
    }
  }
}
```

