from pathlib import Path

import os
import re
import yaml
import constants
import argparse
import shutil
import xml.etree.ElementTree as ET

def _get_testsuites(files):
    test_suites = []
    for f in list(Path(constants.tests_dir).rglob("*.yaml")):
        with open(f) as stream:
            try:
                ts = yaml.safe_load(stream)
                ts['updated'] = False
                if str(f) in files: 
                    ts['updated'] = True
                test_suites.append(ts)
            except yaml.YAMLError as exc:
                print(exc)
                continue
    return test_suites

def _get_test_id_name_map(logs_dir):
    test_id_map = {}
    with open(os.path.join(logs_dir, 'index.html')) as fp:
        for line in fp.readlines():
            matches = re.match("<div><a\shref=\".*\/tests_logs/(.*)\">(.*)</a></div>", line)
            if len(matches.groups()) == 2:
                id, name = [x.strip() for x in matches.groups()]
                name = re.sub(r'\((I-)?(BR|PR)#.*?\)', '', name).strip()
                name = name.replace('(Deviation)', '').strip()
                name = name.replace('(MP)', '').strip()
                name = name.strip()
                name = ' '.join(name.split()[1:])
                test_id_map[name] = id
    return test_id_map

def _did_fail(log_file):
    with open(log_file, 'r') as file:
        data = file.read()
        if data.find('failures="0"') == -1: 
            return True
    return False

def _did_pass(log_file):
    return not _did_fail(log_file)

def _get_test_pkg(tree):
    for p in tree.findall(".//property"):
        if p.get('name') == 'test.path':
            return p.get('value')

def _update_properties(tree, test_log_file, properties):
    changes_needed = {}
    for p in tree.findall(".//property"):
        if p.get('name') in properties:
            p_name = p.get('name')
            p_old_val = p.get('value')
            p_new_val = properties[p_name]
            changes_needed[f'<property name="{p_name}" value="{p_old_val}"></property>'] = f'<property name="{p_name}" value="{p_new_val}"></property>'
            changes_needed[f'*** PROPERTY: {p_name} -> {p_old_val}'] = f'*** PROPERTY: {p_name} -> {p_new_val}'
    
    if len(changes_needed) == 0:
        return

    with open(test_log_file, 'r') as fp:
        contents = fp.read()

    for k in changes_needed:
        contents = contents.replace(k, changes_needed[k])

    with open(test_log_file, 'w') as fp:
        fp.write(contents)

parser = argparse.ArgumentParser(description='Generate MD FireX report')
parser.add_argument('test_suites', help='Testsuite files')
parser.add_argument('firex_ids', help='FireX run IDs')
parser.add_argument('out_dir', help='Output directory')
parser.add_argument('--patches', default=False, action='store_true', help="include patches")
parser.add_argument('--patched-only', default=False, action='store_true', help="skip patched tests")
parser.add_argument('--skip-patched', default=False, action='store_true', help="skip patched tests")
parser.add_argument('--update_failed', default=False, action='store_true', help="update failed tests only")
parser.add_argument('--set-property', action='store', type=str, nargs='*')
args = parser.parse_args()

testsuite_files = args.test_suites
firex_ids= args.firex_ids
out_dir = args.out_dir
include_patches = args.patches
skip_patched = args.skip_patched
patched_only = args.patched_only
update_failed = args.update_failed
set_properties = args.set_property

for firex_id in firex_ids.split(','):
    logs_dir = os.path.join(constants.base_logs_dir, firex_id, 'tests_logs')
    test_id_map = _get_test_id_name_map(logs_dir)

    properties = {}
    if set_properties != None:
        for p in set_properties:
            k, v = p.split('=')
            properties[k] = v

    for ts in  _get_testsuites(testsuite_files.split(',')):
        for t in ts['tests']:
            if t['name'] in test_id_map:
                test_id = test_id_map[t['name']]
                log_files = [str(p) for p in Path(logs_dir).glob(f"{test_id}/ondatra_logs.xml")]
                if len(log_files) == 0: 
                    continue

                tree = ET.parse(log_files[0])
                test_out_dir = os.path.join(out_dir, _get_test_pkg(tree))
                test_log_file = os.path.join(test_out_dir, "test.xml")

                if patched_only and not ('branch' in t or 'pr' in t):
                    continue
                
                if skip_patched and ('branch' in t or 'pr' in t):
                    print("Skipped " + t['name'] + " because it is patched")
                    continue
                
                if update_failed:
                    if _did_fail(log_files[0]):
                        print("Skipped " + t['name'] + " due to failures")
                        continue
                    
                    if os.path.exists(test_log_file) and _did_pass(test_log_file):
                        print("Skipped " + t['name'] + " since it is passing")
                        continue

                print("Adding " + t['name'])
                os.makedirs(test_out_dir, exist_ok=True)
                shutil.copyfile(log_files[0], test_log_file)
                _update_properties(tree, test_log_file, properties)
                # if include_patches and 'patch' in t and os.path.exists(t['patch']):
                #     shutil.copyfile(t['patch'], os.path.join(test_out_dir, "test.patch"))
