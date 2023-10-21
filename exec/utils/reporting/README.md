## Generating XML logs for Google
## Jenkins Flow
https://engci-jenkins-sjc.cisco.com/jenkins/job/team_googleb4/job/CI/job/generate_google_logs/
## Manually 
### Installation
python3 with pyyaml package is the only requirement

```
python3 -m venv .venv
source .venv/bin/activate
pip install pyyaml
```

### Generating logs
```
python exec/utils/reporting/google_reporter.py exec/tests/v2/fp_published.yaml <FireX_ID_1,FireX_ID_2,..> <output_dir>
```

This will collect the logs from multiple FireX runs. Note that the FireX runs are processed in the order
they are provided in the argument list. By default, the logs for a test in `FireX_ID_2` replaces the logs for the same test in `FireX_ID_1` if and only if the test has failed in `FireX_ID_1`. This behaviour can be controlled using mvarious flags (check --help).

### Example
```
python exec/utils/reporting/google_reporter.py exec/tests/v2/fp_published.yaml FireX-arvbaska-230915-051543-46960,FireX-kjahed-230915-012204-24844,FireX-kjahed-230915-233121-52299,FireX-kjahed-230915-175354-32079 out
```

### Sanities
Count the number of tests
```
grep -Rnw <output_dir> -e '<testsuite' | wc -l
```

Count the number of passing tests
```
grep -Rnw <output_dir> -e 'failures="0"' | wc -l
```

### Prepare Archive
```
cd <output_dir> #important
zip -r CISCO.8808.YYYMMDD-HHMM.xxxxxxx.zip feature/*
```
The last 7 characters `xxxxxxx` are the first 7 characters of the FP commit ID used for the runs. It can
be found by looking for the value of the property 'git.commit' in any log file. Example file name `CISCO.8808.20230916-0600.e4897c7.zip`.
