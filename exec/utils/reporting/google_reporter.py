from pathlib import Path

import os
import re
import yaml
import constants
import argparse
import shutil

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
                name = name.replace('(Patched)', '').strip()
                name = name.replace('(Deviation)', '').strip()
                name = name.replace('(MP)', '').strip()
                name = name.strip()
                name = ' '.join(name.split()[1:])
                test_id_map[name] = id
    return test_id_map

parser = argparse.ArgumentParser(description='Generate MD FireX report')
parser.add_argument('test_suites', help='Testsuite files')
parser.add_argument('firex_ids', help='FireX run IDs')
parser.add_argument('out_dir', help='Output directory')
parser.add_argument('--patches', default=False, action='store_true', help="include patches")
args = parser.parse_args()

testsuite_files = args.test_suites
firex_ids= args.firex_ids
out_dir = args.out_dir
include_patches = args.patches

for firex_id in firex_ids.split(','):
    logs_dir = os.path.join(constants.base_logs_dir, firex_id, 'tests_logs')
    test_id_map = _get_test_id_name_map(logs_dir)

    for ts in  _get_testsuites(testsuite_files.split(',')):
        for t in ts['tests']:
            if t['name'] in test_id_map:
                test_id = test_id_map[t['name']]
                log_files = [str(p) for p in Path(logs_dir).glob(f"{test_id}/ondatra_logs.xml")]
                if len(log_files) == 0: 
                    continue
                test_out_dir = os.path.join(out_dir, t['path'])
                os.makedirs(test_out_dir, exist_ok=True)
                shutil.copyfile(log_files[0], os.path.join(test_out_dir, "test.xml"))
                if include_patches and 'patch' in t and os.path.exists(t['patch']):
                    shutil.copyfile(t['patch'], os.path.join(test_out_dir, "test.patch"))