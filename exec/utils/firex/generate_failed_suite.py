import os
import re
import sys
import subprocess
import xml.etree.ElementTree as ET
import argparse
import yaml

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
    new_suite = {}
    suite_index=0

    log_dir = get_run_log_dir(firex_id)

    try:
        with open(f"{log_dir}/all_suites_xunit_results.xml", "r") as fp:
            root = ET.parse(fp)
        testsuite_skuids = []
        for suite in root.findall('testsuite'):
            tests = int(suite.get('tests', 0))
            failures = int(suite.get('failures', 0))
            errors = int(suite.get('errors', 0))
            skipped = int(suite.get('skipped', 0))
            
            if not failed_only or (failures + errors > 0) or (skipped == tests):
                test_dir = None
                test_skuid = None

                properties = suite.find('properties')
                if properties is not None:
                    for prop in properties.findall('property'):
                        if prop.get('name') == 'test_log_directory':
                            test_dir = prop.get('value')
                        if prop.get('name') == 'skuid':
                            test_skuid = prop.get('value')
                            testsuite_skuids.append(prop.get('value'))

                if test_dir:
                    suite_file_path = f"{log_dir}/{test_dir}/testsuite_registration.yaml"
                    
                    # If suite_file_path doesn't exist, try reading from testsuite_metadata_dir
                    if not os.path.exists(suite_file_path):
                        metadata_yaml = f"{log_dir}/{test_dir}/testsuite_metadata_dir/yaml_path.yaml"
                        if os.path.exists(metadata_yaml):
                            with open(metadata_yaml, "r") as meta_file:
                                for line in meta_file:
                                    if line.strip().startswith("yaml_path:"):
                                        suite_file_path = line.split("yaml_path:", 1)[1].strip()
                                        break
                    
                    if os.path.exists(suite_file_path):
                        # Parse the YAML file
                        with open(suite_file_path, "r") as yaml_file:
                            suite_data = yaml.safe_load(yaml_file)
                        
                        # Go over all entries and find ones with matching test_skuid
                        if suite_data:
                            for suite_name, suite_config in suite_data.items():
                                for script_path_item in suite_config.get('script_paths', []):
                                    for path_key in script_path_item:
                                        if path_key == test_skuid:
                                            new_suite[f'{suite_name}_{suite_index}'] = suite_config
                                            suite_index += 1
                    
    except Exception as e:
        raise Exception(f"Error parsing JUnit XML file: {str(e)}")

    if new_suite:
        if out_file:
            with open(out_file, "w") as file:
                yaml.dump(new_suite, file, default_flow_style=False, sort_keys=False)
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
    
