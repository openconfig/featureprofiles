# Openconfig ACL Copp

## Summary 

These tests deal with the configuration of ACL policies and defined sets using openconfig models.

## Procedure

* Establish Gnmi client with ondatra

* Run the following test types for each of the paths listed below

    * update:
        * update
        * bad key
        * invalid
    * replace:
        * replace
        * bad key
        * invalid
    * get:
        * subscribe (verify previous tests using this)
        * get (compatibility), no verification

/system/control-plane-traffic/ingress/acl/acl-set/

/defined-sets/ipv4-prefix-sets/ipv4-prefix-set
/defined-sets/ipv6-prefix-sets/ipv6-prefix-set
/defined-sets/port-sets/port-set

/acl/acl-sets/acl-set/acl-entries/acl-entry
/acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4
/acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6
/acl/acl-sets/acl-set/acl-entries/acl-entry/transport


The below paths are not included in this feature and thus are not tested

*unsupported leafs*:
/oc-sys:system/control-plane-traffic/ingress/acl/matched-octets
/oc-sys:system/control-plane-traffic/ingress/qos
/oc-sys:system/control-plane-traffic/ingress/egress

*unsupported mixed ACL type*: 
Because of the way ACL policies are implemented in iosxr, the ACL type 
`oc.Acl_ACL_TYPE_ACL_MIXED` (located at `github.com/openconfig/ondatra/gnmi/oc`) (a.k.a. `ACL_MIXED`) is unsupported
