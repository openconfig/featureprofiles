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
  use Net::IP;
  $exitcode = 0;
}

END {
  if ($exitcode) {
    print STDERR <<'END'

Error: detected usage of IPv4 and IPv6 addresses outside of the
documentation range.  Please see "IP Addresses Assignment" in
CONTRIBUTING.md for detail.
END
  }
  exit $exitcode;
}

# Reset line number for each input file: https://perldoc.perl.org/perlfunc#eof
close ARGV if eof;  # Not eof()!

my $lineok = 1;

# IPv4
if (/\b(\d{1,3}(\.\d{1,3}){3,})(\/\d+)?\b/) {
  my $parsed = new Net::IP($1);
  next if !$parsed;                     # Not parsed as an IPv4.

  my $ip = $parsed->ip();
  next if $ip =~ /192\.0\.2\./;         # TEST-NET-1 (RFC 5737)
  next if $ip =~ /198\.51\.100\./;      # TEST-NET-2 (RFC 5737)
  next if $ip =~ /203\.0\.113\./;       # TEST-NET-3 (RFC 5737)
  next if $ip =~ /198\.(18|19)\./;      # BMWG (RFC 2544)
  next if $ip =~ /20\.0\./;             # 20.0.0.1/15
  next if $ip =~ /30\.0\./;             # 30.0.0.1/15
  next if $ip =~ /100\.0\./;            # 100.0.0.1/12
  next if $ip =~ /138\.0\.11\./;        # 138.0.11.0/24
  next if $ip =~ /192\.51\.100\./;      # 192.51.100.1/24
  next if $ip =~ /192\.51\.128\./;      # 192.51.129.0/22
  next if $ip =~ /192\.55\.200\./;      # 192.55.200.3/32
  next if $ip =~ /198\.100\.200\./;     # 198.100.200.123/24
  next if $ip =~ /192\.58\.200\./;      # 192.58.200.1/24
  next if $ip =~ /203\.10\.113\./;      # 203.10.113.1/24
  next if $ip =~ /192\.51\.129\./;      # 192.51.129.0/22
  next if $ip =~ /192\.168\.10\./;      # 192.168.10.0/24
  next if $ip =~ /192\.168\.20\./;      # 192.168.20.0/24
  next if $ip == "0.0.0.0";             # Wildcard
  $lineok = 0;
}

# IPv6
if (/\b(([[:xdigit:]]+(:|::)){2,}[[:xdigit:]]*)(\/\d+)?\b/) {
  my $parsed = new Net::IP($1);
  next if !$parsed;                     # Not parsed as an IPv4.

  my $ip = $parsed->ip();
  next if $ip =~ /2001:0?db8:/i;        # IPv6 Test Net (RFC 3849)
  next if $ip =~ /fe80:/i;              # IPv6 Link Local
  $lineok = 0;
}

if (!$lineok) {
  print "$ARGV:$NR: $_";
  $exitcode = 1;
}
