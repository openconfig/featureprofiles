## P4RT Implementation Guide

This document specifies the requirements for p4rt test implementation.```

1.  Use the [antoninbas P4RT client](https://github.com/antoninbas/p4runtime-go-client).

2.  The client should import or make use of the following WBB information in the
    following Google compatible format:

    1.  WBB P4 info to generate static P4Info:
        https://github.com/pins/pins-infra/blob/master/sai_p4/instantiations/google/wbb.p4info.go

    2.  WBB P4 Protobuf file:
        https://github.com/pins/pins-infra/blob/master/sai_p4/instantiations/google/wbb.p4info.pb.txt

    We are working on getting these two files open sourced meanwhile.

3.  The client should include a library to make use of Ondatra Raw APIs while
    it's being implemented. This is compulsory for us to easily adapt into our
    current testbeds. 
