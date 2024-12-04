import os
import re
import sys
import subprocess
import xml.etree.ElementTree as ET

def get_run_log_dir(firex_id):
    try:
        out = subprocess.check_output(f"/auto/firex/bin/firex find {firex_id}", shell=True, universal_newlines=True).strip()
        
        matcher = re.search(r"(?m)^.*Logs: (.*)", out)
        if matcher:
            log_dir = matcher.group(1).strip()
            return log_dir
        else:
            raise Exception(f"Log directory not found for FireX run {firex_id}")
    except subprocess.CalledProcessError as e:
        raise Exception(f"Error executing shell command: {e}")

def write_failed_tests_suite(firex_id, out_file=None):
    new_suite = ""
    log_dir = get_run_log_dir(firex_id)

    try:
        with open(f"{log_dir}/all_suites_xunit_results.xml", "r") as fp:
            root = ET.parse(fp)

        suite_index = 0
        testsuite_names = []
        for suite in root.findall('testsuite'):
            failures = int(suite.get('failures', 0))
            errors = int(suite.get('errors', 0))
            if failures + errors > 0:
                test_dir = None

                properties = suite.find('properties')
                if properties is not None:
                    for prop in properties.findall('property'):
                        if prop.get('name') == 'test_log_directory':
                            test_dir = prop.get('value')
                        if prop.get('name') == 'testsuite_name':
                            testsuite_names.append(prop.get('value'))

                if test_dir:
                    suite_file_path = f"{log_dir}/{test_dir}/testsuite_registration.yaml"
                    if os.path.exists(suite_file_path):
                        with open(suite_file_path, "r") as suite_file:
                            lines = suite_file.readlines()
                            for line in lines:
                                if re.match(r'^\S+:', line):
                                    line = re.sub(r'^(\S+):', rf'\1_{suite_index}:', line)
                                    suite_index+=1
                                new_suite += line
    except Exception as e:
        raise Exception(f"Error parsing JUnit XML file: {str(e)}")

    if new_suite.strip():
        out_file = out_file if out_file else f"{firex_id}_failed_suites.yaml"
        with open(out_file, "w") as file:
            file.write(new_suite)
        print(f"{','.join(testsuite_names)}")
    else:
        raise Exception(f"No failed suites found in run {firex_id}")

# Example of how to call the function
if __name__ == "__main__":
    out_file = None if len(sys.argv) < 3 else sys.argv[2]
    write_failed_tests_suite(sys.argv[1], out_file)
