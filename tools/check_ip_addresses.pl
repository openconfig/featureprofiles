#!/usr/bin/perl -ln
#
# This script checks that usage of IP addresses in *.go code adheres
# to the contributor guide:
#
# https://github.com/openconfig/featureprofiles/blob/main/CONTRIBUTING.md#ip-addresses-assignment
#
# Implementation note: perl -ln wraps this script body to be executed
# per line of input, like awk.  BEGIN and END still only run at the
# very beginning and end of the execution, rather than per line.
#
# See: https://perldoc.perl.org/perlvar

BEGIN {
  use English;
  $exitcode = 0;
}

END {
  exit $exitcode;
}

# Reset line number for each input file: https://perldoc.perl.org/perlfunc#eof
close ARGV if eof;  # Not eof()!

my $lineok = 1;

# IPv4
if (/\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})(\/\d+)?\b/) {
  next if $1 =~ /192\.0\.2\./;          # TEST-NET-1
  next if $1 =~ /198\.51\.100\./;       # TEST-NET-2
  next if $1 =~ /203\.0\.113\./;        # TEST-NET-3
  next if $1 == "0.0.0.0";              # Wildcard
  $lineok = 0;
}

# IPv6
if (/\b(([[:xdigit:]]+(:|::)){2,}[[:xdigit:]]*)(\/\d+)?\b/) {
  next if $1 =~ /2001:0?db8:/i;         # IPv6 Test Net
  next if $1 =~ /([[:xdigit:]]{2}:){5}[[:xdigit:]]{2}/;  # MAC addresses.
  next if $_ =~ /02:1a:WW:XX:YY:ZZ/;    # MAC address used in example.
  next if $1 =~ /\d{2}:\d{2}:\d{2}/;    # Time.
  $lineok = 0;
}

if (!$lineok) {
  print "$ARGV:$NR: $_";
  $exitcode = 1;
}
