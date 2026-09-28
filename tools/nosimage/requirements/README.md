# NOSImageProfile Enhancement Requirements

The `NOSImageProfile` describes a network operating system's (NOS) properties, including identifying the software, its support of OpenConfig RPCs and paths, as well as an assertion of [featureprofiles](https://github.com/openconfig/featureprofiles/tree/main) test results.

## Enhancement Field Requirements

### 1. Machine-Consumable featureprofiles Test Data
To enable automated coverage analysis and validation tracking, vendors must provide structured test results within the `featureprofile_test_result` field. This structured reporting replaces manual or unstructured updates, providing a unified view of test outcomes.

*   **Field**: `repeated FeatureProfileTestResult featureprofile_test_result = 8;`
*   **Attributes**:
    *   **plan_id**: The unique identifier for the test plan, which must match the metadata defined in the `featureprofiles` repository.
    *   **commit**: The specific git commit hash of the `featureprofiles` repository used for the execution.
    *   **result**: The final status of the test (e.g., `PASSED`, `FAILED`, `NOT_EXECUTED`).

### 2. Image Release Type
The `image_type` field defines the release stage of the network operating system image, allowing automated ingestion pipelines to categorize builds correctly.

* **Field**: `ImageType image_type = 10;`
* **Supported Values**:
  * `IMAGETYPE_GA`: General Availability release.
  * `IMAGETYPE_EFT`: Early Field Trial release.


---

## Textproto Example

```textproto
vendor_id: OPENCONFIG
nos: "network-os"
software_version: "24.1.R1"
hardware_name: "fixed-sku-01"

# Enhancement: Multiple machine-consumable test results
featureprofile_test_result: {
  plan_id: "interfaces-base-config"
  commit: "a1b2c3d4e5f6g7h8i9j0"
  result: PASSED
}
featureprofile_test_result: {
  plan_id: "bgp-neighbor-state"
  commit: "a1b2c3d4e5f6g7h8i9j0"
  result: FAILED
}



# Enhancement: Categorized image release type
image_type: IMAGETYPE_GA