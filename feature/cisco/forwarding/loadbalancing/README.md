# Loadbalancing test for different hashing parameters using Google DVT topology

## Testbed type
DVT SIM topology Testbed ID: DVT-SIM-TOPO

## Summary

This test suite is to test Loadbalancing tests using Google DVT topology,
with different hashing parameters 
    * algorithm-adjust
    * RTF profiles/swizzle
    * extended-entropy
    * ECMP/SPA seed

## Procedure

*   TODO


## Feature Info and documents 
```
FEAT: FEAT-37197 Google: Avoid Polarization via Dupe+Masking of fields
SFS/Scoping Doc: https://cisco-my.sharepoint.com/:x:/p/arvbaska/ETLBkY66uNpCgSkmxKQj7YIBC8F636MazdoROFayce1ixg
EDCS:
WIKI/TOI: https://wiki.cisco.com/display/SFFWD/Spitfire+CEF+Load+balancing%3A+Per+NPU+Hash+Rotate+feature
MISC: 
```

## OpenConfig Path and RPC Coverage

Not applicable

```yaml
paths:
  ## Config Paths ##

  ## State Paths ##


rpcs:
  gnmi:
    gNMI.Subscribe:
```

## Control Protocol Coverage

BGP
IS-IS
gRIBI

## Minimum DUT Platform Requirement

8000 Q200
