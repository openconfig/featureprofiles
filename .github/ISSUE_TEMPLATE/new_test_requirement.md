---
name: New featureprofiles test requirement
about: Use this template to document the requirements for a new test to be implemented.
title: ''
labels: enhancement
assignees: ''

---

# TestID-x.y: Short name of test here

## Summary

Write a few sentences or paragraphs describing the purpose and scope of the test. 

## Procedure

*   Test #1 - Name of test
    *   Step 1
    *   Step 2
    *   Step 3

*   Test #2 - New of test
    *   Step 1
    *   Step 2
    *   Step 3


## Config Parameter Coverage

Add list of OpenConfig 'config' paths used in this test, if any.

## Telemetry Parameter Coverage

Add list of OpenConfig 'state' paths used in this test, if any.

## Protocol/RPC Parameter Coverage

Add list of OpenConfig RPC's (gNMI, gNOI, gNSI, gRIBI) used in the list

For example:
*   gNMI
    *   Set
    *   Subscribe
*   gNOI
    *   System
        *   KillProcess
    *   Healthz
        *   Get
        *   Check
        *   Artifact
