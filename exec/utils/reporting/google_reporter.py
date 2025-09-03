from pathlib import Path
from glob import glob

import os
import constants
import argparse
import shutil
import xml.etree.ElementTree as ET

def _did_fail(log_file):
    try:
        with open(log_file, 'r') as file:
            data = file.read()
            if data.find('failures="0"') == -1: 
                return True
    except:
        print("Error processing file " + log_file)
        return True
    return False

def _did_pass(log_file):
    return not _did_fail(log_file)

parser = argparse.ArgumentParser(description='Generate MD FireX report')
parser.add_argument('firex_ids', help='FireX run IDs')
parser.add_argument('out_dir', help='Output directory')
parser.add_argument('--patches', default=False, action='store_true', help="include patches")
parser.add_argument('--patched-only', default=False, action='store_true', help="skip patched tests")
parser.add_argument('--passed-only', default=False, action='store_true', help="skip failed tests")
parser.add_argument('--skip-patched', default=False, action='store_true', help="skip patched tests")
parser.add_argument('--update-failed', default=True, action='store_true', help="update failed tests only")
args = parser.parse_args()

firex_ids= args.firex_ids
out_dir = args.out_dir
include_patches = args.patches
skip_patched = args.skip_patched
passed_only = args.passed_only
patched_only = args.patched_only
update_failed = args.update_failed

total_count = 0
for firex_id in firex_ids.split(','):
    print("Processing " + firex_id)
    uname = firex_id.split('-')[1]
    logs_dir = ''
    for loc in constants.logs_locations:
        for group in constants.test_groups + [uname]:
            p = os.path.join(loc, group, firex_id, 'tests_logs')
            if os.path.exists(p):
                logs_dir = p

    if logs_dir:
        print(f"Found logs directory: {logs_dir}")
    else:
        print(f"Coult not find log directory for run {firex_id}")
        os.exit(1)
    
    
    for log_file in glob(os.path.join(logs_dir, '*/ondatra_logs.xml'), recursive=True):
        print(f"Processing {log_file}")   
        try:
            tree = ET.parse(log_file)
        except:
            print(f"Error parsing {log_file}. Skipping...")
            continue
        
        test_path = ''
        test_plan = ''
        for p in tree.findall(".//property"):
            if p.get('name') == 'test.path':
                test_path = p.get('value')
            elif p.get('name') == 'test.plan_id':
                test_plan = p.get('value')

        if not test_path or not test_plan:
            print(f"Could not find test path. Skipping...")
            continue

        test_str = f"{test_path} ({test_plan})"
        test_out_dir = os.path.join(out_dir, test_path)
        test_log_file = os.path.join(test_out_dir, "test.xml")

        if not os.path.exists(test_log_file) or (_did_fail(test_log_file) and _did_pass(log_files)):
            print(f"Adding {test_str}")
            total_count += 1
            os.makedirs(test_out_dir, exist_ok=True)
            shutil.copyfile(log_file, test_log_file)

print(f"Added {total_count} tests")
