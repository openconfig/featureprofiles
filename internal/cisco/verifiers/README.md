# XR Verifiers

Common test verifiers for different XR components using either OC Yang state paths,
Use show CLIs/native yang where OC path is not applicable or supported.


## OC Yang

#### Guidelines:

## CLIs

#### Guidelines:

* Leverage already available textFsm templates developed as part of cafy test infra 
https://gh-xr.scm.engit.cisco.com/xr/iosxr/tree/main/test/kit/lib/parsers/xr/templates

* TextFsm templates can added under featureprofiles/internal/cisco/verifiers/textfsm_templates directory.
Create any sub directories as necessary.

#TODO : Parsing textFsm with https://github.com/sirikothe/gotextfsm package
