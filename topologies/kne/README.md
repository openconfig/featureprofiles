
- The .textproto files in this directory are used for creating kne topologies with various elements. These may contain one or more DUTs with ports connected to other DUTs or OTG ports
- There are also DUT configuration files which are used during deployment and reset phase (When a test is started, the ondatra reservation will perform a DUT reset).
- One will need to have all the docker images available required in the textproto file and the "operator" pods up and running. See this [kne/kind-bridge example](https://github.com/openconfig/kne/blob/main/deploy/kne/kind-bridge.yaml)
- Before deploying a topology requiring otg ports, one will need to apply to the cluster the ixiatg-configmap.yaml file. Every release could be found [here](https://github.com/open-traffic-generator/ixia-c/releases)
- Then the version from the .textproto file must be changed from "0.0.1-9999" to the one used by the this config map file before the actual kne topology creation. (e.g _**version: "0.0.1-9999"**_ into _**version: "0.0.1-3662"**_ )
- Examples on how to create kne topologies can be found [here](https://github.com/openconfig/kne/tree/main/examples)
