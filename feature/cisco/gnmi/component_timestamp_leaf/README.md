# last_reboot_time: Google: Update behavior of component timestamp leafs to latest

Previous behavior of the following leaf was to updated to match the description of boot-time leaf in version of 0.31.0.
/openconfig-platform:components/component/state/last-reboot-time
Related Openconfig PR
https://github.com/openconfig/public/pull/1179
This feature does not include uprev of the model.

Once model uprev occurs, this leaf will be deprecated:
- /openconfig-platform:components/component/state/last-reboot-time

and be moved to:
- /openconfig-platform:components/component/state/boot-time


Accompanying tests ensure the leaf is functional based on different types of hardware reloads. Including both COLD and WARM reboot methods on RPs and LCs

CRD: EDCS-25527565 
