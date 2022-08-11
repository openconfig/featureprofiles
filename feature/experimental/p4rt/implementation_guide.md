## P4RT Implementation Guide

With a variety of options that we have for P4RT clients and after a careful
consideration we would like to take it to the next level in getting a PR for at
least one feature profile test case that satisfies all of the following
conditons:

1.  Makes use of antoninbas's P4RT client exclusively and adds all functionality
    that this client claims of supporting.

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

4.  Additionally, the client should support all features like Packet I/O, Master
    Arbitration, Flow programming, counter statistics, send/punt packets with
    the above mentioned P4 info etc. We require all the clients to be using
    Apache 2.0 as the standard license when they are opensourced. 
