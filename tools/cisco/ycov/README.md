# YANG Coverage 

Support for XR Yang Coverage gathering and reporting through go test. \
Refer: https://wiki.cisco.com/display/XRMGBLMOVE/Yang+Data-Model+Coverage

**Default YANG coverage flow is -**
1. Pretest - Clear and Enable logs
2. Testsuite Run
3. Post Test - \
    3.1 Gather logs \
    3.2 If xr_ws flag is passed and accessible along with xr options, process raw logs and generate reports else store raw logs.

## Enable YANG Coverage
 One needs to setup and execute test as mentioned below -

#### Local Run:
1. Edit [YCov textproto](conf/fp_public_ycov.textproto)
2. Copy, edit and run below ycov_runner.sh
```
cat ycov_runner.sh
go test -v ./tools/cisco/ycov/pretest/... -testbed <testbed_path> -binding <binding_path> -yang_coverage ../conf/fp_public_ycov.textproto  -logtostderr=true  -xr_ws <xr_ws_root_path>
go test -v ./feature/cisco/bgp/... -testbed <testbed_path> -binding <binding_path>
go test -v ./tools/cisco/ycov/posttest/... -testbed <testbed_path> -binding <binding_path> -yang_coverage ../conf/fp_public_ycov.textproto  -logtostderr=true  -xr_ws <xr_ws_root_path>
```

```
cd featureprofiles/
./ycov_runner.sh
```
Output Sample:
```
Coverage logs stored at /ws/ncorran-sjc/yang-coverage//2023_05_03__22_41_13_FeatureProfilesPublic_PRECOMMIT_validated.json.
YCov tool logs at /nobackup/sanshety/ws/iosxr/2023_05_03__22_41_13_FeatureProfilesPublic_PRECOMMIT_ycov.log
```

#### FireX Integration:
To enable and collect Yang Coverage logs, need to add YCov Pretest with priority 0 and YCov Post test with lowest priority. \
Refer: [fp_published.yaml](../../../exec/tests/v2/fp_published.yaml)

## YCov Flags
Below flags can be passed for YCov Pretest and Post test:
| **Flag**            | **Description**                                                     | **Default**                            |
|---------------------|---------------------------------------------------------------------|----------------------------------------|
| -yang_coverage      | Yang coverage configuration file.                                   | ../conf/fp_public_ycov.textproto       |
| -xr_ws              | XR workspace path.                                                  |                                        |
| -subcomp            | XR subcomponent name to be targeted for coverage analysis.          |                                        |
| -mgblPath           | Location where the analysis result will be saved for extra analysis.| /ws/ncorran-sjc/yang-coverage/         |
| -rawLogsPath        | Location where the raw coverage data will be saved for analysis".   | /ws/ncorran-sjc/yang-coverage/rawlogs/ |
