from datetime import datetime
import json
import hashlib
from pathlib import Path
from gotest2html import GoTestSuite, parse_json

import os
import re
import yaml
import constants
import argparse

def _get_testsuites():
    test_suites = []

    for f in list(Path(constants.tests_dir).rglob("*.yaml")):
        with open(f) as stream:
            try:
                ts = yaml.safe_load(stream)
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
                name = ' '.join(name.split()[1:])
                test_id_map[name] = id
    return test_id_map

parser = argparse.ArgumentParser(description='Generate MD FireX report')
parser.add_argument('firex_id', help='FireX run ID')
parser.add_argument('out_dir', help='Output directory')
args = parser.parse_args()

firex_id = args.firex_id
out_dir = args.out_dir
data_dir = os.path.join(out_dir, constants.gh_data_dir)
gh_logs_dir = os.path.join(out_dir, constants.gh_logs_dir)
gh_reports_dir = os.path.join(out_dir, constants.gh_reports_dir)

if not os.path.exists(data_dir):
    os.makedirs(data_dir)
if not os.path.exists(gh_logs_dir):
    os.makedirs(gh_logs_dir)
if not os.path.exists(gh_reports_dir):
    os.makedirs(gh_reports_dir)

now = datetime.now().timestamp()
logs_dir = os.path.join(constants.base_logs_dir, firex_id, 'tests_logs')

summary_md = """
## Summary
Total | Passed | Failed | Regressed | Skipped
------|--------|--------|-----------|--------
"""
details_md = """
## Test Suites
Suite | Total | Passed | Failed | Regressed | Skipped | Last Run | Logs | Result
------|-------|--------|--------|-----------|---------|----------|------|-------
"""

total, passed, failed, skipped, regressed = [0] * 5
test_id_map = _get_test_id_name_map(logs_dir)

for ts in  _get_testsuites():
    ts_data_file = os.path.join(data_dir, f"{ts['name']}.json")
    ts_html_file = os.path.join(gh_reports_dir, f"{ts['name']}.html")

    if os.path.exists(ts_data_file):
        with open(ts_data_file, 'r') as fp:
            go_test_suite = GoTestSuite.from_json_obj(json.loads(fp.read()))
    else:
        go_test_suite = GoTestSuite(run_id=firex_id, last_updated=now)

    go_tests = []
    for t in ts['tests']:
        if t['name'] in test_id_map:
            test_id = test_id_map[t['name']]
            log_files = [str(p) for p in Path(logs_dir).glob(f"{test_id}/*.json")]
            try:
                gt = parse_json(log_files[0], suite_name=t['name'])
                go_tests.append(gt)
            except: continue
    
    if len(go_tests) > 0:
        go_test_suite.update(firex_id, go_tests, last_updated=now)
        with open(os.path.join(out_dir, f"{ts['name']}.md"), 'w') as fp:
            fp.write(go_test_suite.to_md_string())

        with open(ts_data_file, 'w') as fp:
            fp.write(json.dumps(go_test_suite.to_json_obj()))

        for t in go_tests:
            for c in t.get_descendants() + [t]:
                with open(os.path.join(gh_logs_dir, c.get_log_file_name()), 'w') as fp:
                    fp.write(c.get_output())

    suite_stats = go_test_suite.get_stats()
    if suite_stats['total'] == 0:
        continue

    total += suite_stats['total']
    passed += suite_stats['passed']
    failed += suite_stats['failed']
    skipped += suite_stats['skipped']
    regressed += suite_stats['regressed']

    suite_time = datetime.fromtimestamp(go_test_suite.get_last_updated()).strftime("%y-%m-%d %H:%M")
    suite_results = ''
    if suite_stats['regressed'] > 0: suite_results = ':warning:'
    elif suite_stats['failed'] > 0: suite_results = ':x:'
    elif suite_stats['total'] > 0: suite_results = ':white_check_mark:'

    details_md += f"[{ts['name']}]({ts['name']}.md)|{suite_stats['total']}|{suite_stats['passed']}|{suite_stats['failed']}"
    details_md += f"|{suite_stats['regressed']}|{suite_stats['skipped']}|[{suite_time}]({constants.base_tracker_url}{go_test_suite.get_last_run_id()})"
    details_md += f"|[HTML]({constants.gh_repo_raw_url}/{constants.gh_reports_dir}/{ts['name']}.html) [RAW]({constants.base_logs_url}{go_test_suite.get_last_run_id()}/tests_logs/)|{suite_results}\n"

    with open(ts_html_file, 'w') as fp:
        fp.write(go_test_suite.to_html())

summary_md += f"{total}|{passed}|{failed}|{regressed}|{skipped}\n"

with open(os.path.join(out_dir, f"README.md"), 'w') as fp:
    fp.write(summary_md + details_md)