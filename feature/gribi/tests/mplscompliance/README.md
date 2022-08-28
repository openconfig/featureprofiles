# gRIBI MPLS Compliance Tests

This feature profile covers the programming of MPLS entries via the gRIBI API.

## Test Cases

1. Push a label stack to an existing MPLS label stack.
  * forward packet destined to label 100
  * program a LabelEntry instructing the DUT to push a label stack consisting
    of 1 <= x <= 20 labels to this stack.
