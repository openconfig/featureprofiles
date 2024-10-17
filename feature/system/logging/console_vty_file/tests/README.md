# TR-6.2 Local logging destinations

## Sumary
Verify if required OC configuration is accepted by DUT and exposed via gNMI get

## Prodecure

### Configuration

### TC1 - console logging configuration

1. configure and enable consloe logging with:
    - 2 selectors:
      - facility = "Local7", severity = "informational"
      - facility = "Local6", severity = "alarm"
      - facility = "Local5", severity = "critical"
    - remote facility rewrites:
      - "local7" --> "local2"
      - "local6" --> "local3"
  Note: Facility "Local5" is not rewritten
2. Read consloe logging configuration and compate with pushed configuration.\
   Note: two selectors must be presented.
3. disable consloe logging while keeping 2 selectors configured:
4. Read consloe logging configuration and compate with pushed configuration.\
   Note: two selectors must be presented.

### [TODO] TC2 - VTY logging configuration
The vty represents here terminal session - ssh, telnet.
> NOTE: This is NOT yet modeled in OpenConfig

1. configure and enable vty logging with 2 selectors:
    - facility = "Local7", severity = "informational"
    - facility = "Local5", severity = "alarm"
2. Read vty logging configuration and compate with intended configuration.\
   Note: two selectors must be presented.
3. disable vty logging while keeping 2 selectors configured:
4. Read vty logging configuration and compate with intended configuration.\
   Note: two selectors must be presented.

### [TODO] TC2 - buffer logging configuration
> NOTE: This is NOT yet modeled in OpenConfig
1. Configure and enable buffer logging with size set to 5000
2. Read buffer logging configuration and compate with intended configuration.
3. Change buffer logging with size set to 7000
4. Read buffer logging configuration and verify that buffer size changed
5. Disable buffer logging


### [TODO] TC3 - files logging configuration
> NOTE: This is NOT yet modeled in OpenConfig
1. configure and enable file logging with:
    - 2 selectors:
      - facility = "Local7", severity = "informational"
      - facility = "Local6", severity = "alarm"
    - file base name logfile_1
    - directory path
    - log file management: number 3, maximum size 1M, max age 1440min (24h)
    - remote archival to SCP destination
2. configure and enable file logging with:
    - 2 selectors:
      - facility = "Local5", severity = "informational"
      - facility = "Local6", severity = "warning"
    - file base name logfile_2
    - directory path
    - log file management: number 3, maximum size 10M, log file max age 1min
3. configure log file management for implicit logging: number 3, maximum size 1M, log file max age 1440min (4h)
5. Read file logging configuration and compate with intended configuration.
6. Wait 4 minutes and verify number of logfile_2 stored - should be 5

## Configuration Paramenter Coverage
*  /system/logging/console/selectors/selector/config/facility
*  /system/logging/console/selectors/selector/config/severity
*  [TODO: extend OC modle]/system/logging/console/config/enable

## Telemety Paramenter Coverage
*  /system/logging/console/selectors/selector/config/facility
*  /system/logging/console/selectors/selector/config/severity
*  [TODO: extend OC modle]/system/logging/console/config/enable

## DUT
vRX
