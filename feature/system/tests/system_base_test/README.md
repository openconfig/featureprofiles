# System-1: System testing

## Summary

Ensures the device support basic system requirements for a device supporting g* APIs.

## service-1

### Procedure

Each test will require the DUT configured with a basic service configuration that
should be provided as part of the basic configuration.  This setup should also include
any security setup for connecting to the services.

The default setup should expect a CA signed certifate and trust bundle which can be
used for mTLS.

| Protocol  | Port  |
| --------- | ----- |
| gNMI      | 9339  |
| gNOI      | 9339  |
| gNSI      | 9339  |
| gRIBI     | 9340  |
| P4RT      | 9559  |

### Tests

| ID          | Case         | Result          |
| ----------- | ------------ | --------------- |
| service-1.1 | gNMI client  | gNMI Get works  |
| service-1.2 | gNOI client  | gNOI system Time works |
| service-1.3 | gNSI client  | gNSI authz Get works |
| service-1.4 | gRIBI client | gRIBI Get works |
| service-1.5 | p4rt client  | P4RT Capabilities works |

1. Configure DUT with service configurations for all required services
2. Each test will then create a client to those services and valid each service is properly
   listening on the standard port.
3. Validate client properly connects and execute a simple RPC to validate no errors are returned.
