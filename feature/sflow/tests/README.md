# Sampling Tests

## Summary

Test sFlow sampling collection on targets.

### DUT service setup

Configure the DUT to enable the following services

* gNMI

The DUT will be pushed a default configuration which will enable the sFlow collection on the device with the target being specified via the provided `collector_target`

## Tests

### Sflow-1: Sample basic testing

#### Sflow-1.1: Validate collection of samples for traffic

* Traffic Profile

| Traffic Item | PPS | Packet Size | L3  | L4  |
| ------------ | --- | ----------- | --- | --- |
| sflow1  | 1000   | 64   | IP | TCP |
| sflow2  | 10000  | 64   | IP | TCP |
| sflow3  | 100000 | 64   | IP | TCP |
| lflow1  | 1000   | 1500 | IP | TCP |
| lflow2  | 10000  | 1500 | IP | TCP |
| lflow3  | 100000 | 1500 | IP | TCP |
| mflow1  | 1000   | 512 | IP | TCP |
| mflow2  | 10000  | 512 | IP | TCP |
| mflow3  | 100000 | 512 | IP | TCP |

  1. Push configuration to DUT for enabling sFlow on the device for traffic ports.
  2. Push configuration to ATE for traffic flows between ports.
  3. Validate traffic flowing between ports.
     * Traffic above profile profile
     * Each flow should use the same IP SRC/DST and TCP SRC/DST
     * Each flow should be configured for both IPv4 and IPv6
  4. Validate collector is receieving samples from DUT.
  5. Validate collector samples align with configured flows.
  6. Validate gNMI paths for sampling setup.
  7. Validate gNMI state paths.
