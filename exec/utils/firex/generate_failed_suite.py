import os
import re
import sys
import subprocess
import xml.etree.ElementTree as ET
import argparse

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

def write_failed_tests_suite(firex_id, out_file=None, failed_only=True):
    new_suite = ""
    log_dir = get_run_log_dir(firex_id)

    try:
        with open(f"{log_dir}/all_suites_xunit_results.xml", "r") as fp:
            root = ET.parse(fp)

        suite_index = 0
        testsuite_skuids = []
        for suite in root.findall('testsuite'):
            failures = int(suite.get('failures', 0))
            errors = int(suite.get('errors', 0))
            if not failed_only or (failures + errors > 0):
                test_dir = None

                properties = suite.find('properties')
                if properties is not None:
                    for prop in properties.findall('property'):
                        if prop.get('name') == 'test_log_directory':
                            test_dir = prop.get('value')
                        if prop.get('name') == 'skuid':
                            testsuite_skuids.append(prop.get('value'))

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
        if out_file:
            with open(out_file, "w") as file:
                file.write(new_suite)
        else:
            print(f"{','.join(testsuite_skuids)}")
    else:
        raise Exception(f"No suites found in run {firex_id}")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Generate test suite YAML from FireX run.')
    parser.add_argument('firex_id', type=str, help='FireX run ID')
    parser.add_argument('out_file', type=str, nargs='?', help='Output file for the test suite YAML')
    parser.add_argument('--failed_only', action='store_true', help='Only consider failed test suites')
    args = parser.parse_args()

    write_failed_tests_suite(args.firex_id, args.out_file, args.failed_only)
    
