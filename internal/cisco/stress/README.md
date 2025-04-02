## stress utils
Makes use of three CLI tools to stress different resources in the DUT system.

### stress
[stress](https://linux.die.net/man/1/stress) is a linux binary which simulates the use of different resources.
This is used for *CPU* and *Memory* stressing.

> Note this is not installed by default on cisco devices, and will be installed by the util on first call

### fallocate
[fallocate](https://linux.die.net/man/1/fallocate) is a linux binary which allows storage to be allocated to simulate disk usage.
This is used for *Disk0* and *HardDisk* stressing.

### spi_envmon_test
[spi_envmon_test](https://wiki.cisco.com/display/FRE/SPI+TEST+Util+Commands) is a Cisco-inhouse linux binary which allows various sensor values to be simulated.
This is used for *Power* stressing.

> Note this is only available on dev images
