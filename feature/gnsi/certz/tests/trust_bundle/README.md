# Certz-4: Trust Bundle

## Summary

Server and client TLS endpoints use x.509 certificates for
identification of the calling or called endpoint. Systems
could use self-signed certificates and not validate, but
this is an insecure practice.

Servers and clients should require that the certificates
used are validated and are signed by a known Certificate
Authority(CA).

The known CAs which can be used are contained in a
'trust bundle', which is a list of public keys of the CAs.
The list of CA public keys must be kept up to date, as
CAs will rotate their key material on a regular cadence.

CA keys may be one of two valid (at this time) key algorithms:

   * RSA
   * ECDSA

(Note: Security of key algorithms is subject to change, the
system must be able to support more than one key type at any
point in time in order to support key algorithm change events.)

A trust bundle may have one or more certificates contained in
it, systems should be able to support at least twenty thousand
CA keys in such a bundle.


## Baseline Setup

### Input Args

  * the set of certificate testdata generated with the mk_cas.sh
    script in the featureprofiles/feature/security/gnsi/certz/test_data
    directory.

### DUT service setup

Configure the DUT to enable the following sevices (that are using gRPC) are
up and require using mTLS for authentication:

  * gNMI
  * gNOI
  * gNSI
  * gRIBI
  * P4RT

For each trust_bundle created by mk_cas.sh, configure the
services to load the correct key-type certificate, key and
trust_bundle.

## Tests

### Test Certz-4.1

Load the server certificate and key from each of the following CA sets:
   * ca-01
   * ca-02
   * ca-10
   * ca-1000
   * ca-20000

Each service must be configured to use the appropriate certificate and validate
that certificate using the included trust_bundle. Loading a new trust_bundle
should not take longer than 120 seconds.

Perform this test with both RSA dn ECDSA key-types.

## Canonical OC
```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "test interface",
          "name": "port1"
        },
        "name": "port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.20.0.1",
                        "prefix-length": 32
                      },
                      "ip": "192.20.0.1"
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  },
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "GRPC_TEST"
        },
        "interfaces": {
          "interface": [
            {
              "config": {
                "id": "port1",
                "interface": "port1"
              },
              "id": "port1"
            }
          ]
        },
        "name": "GRPC_TEST"
      }
    ]
  },
  "system": {
    "grpc-servers": {
      "grpc-server": [
        {
          "config": {
            "enable": true,
            "name": "gmmi-test",
            "network-instance": "GRPC_TEST",
            "port": 9339,
            "services": [
              "GNMI",
              "GNSI",
              "GRIBI"
            ]
          },
          "name": "gmmi-test"
        },
        {
          "config": {
            "enable": true,
            "name": "p4rt-test",
            "network-instance": "GRPC_TEST",
            "port": 9559,
            "services": [
              "P4RT"
            ]
          },
          "name": "p4rt-test"
        }
      ]
    }
  }
}
```
## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

TODO(OCRPC): Record may not be correct or complete

```yaml
rpcs:
  gnsi:
    certz.v1.Certz.GetProfileList:
```


## Minimum DUT Platform Requirement

vRX
