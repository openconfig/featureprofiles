import os
import pytest
import json
import tempfile
import shutil
import logging
import time

import xml.etree.ElementTree as ET

from InMemoryDatabase import InMemoryDatabase

import sys
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from CIT import CIT

logger = logging.getLogger(__name__)

def create_test_suite(ts):
    test_cases = ts.get('test_cases', [])
    attributes = ts.get('attributes', {})
    properties = ts.get('properties', {})

    tests = len(test_cases)
    failures = sum(1 for test_case in test_cases if "failure" in test_case)
    errors = sum(1 for test_case in test_cases if "error" in test_case)
    skipped = sum(1 for test_case in test_cases if "skipped" in test_case)
    disabled = sum(1 for test_case in test_cases if "disabled" in test_case)

    testsuite = ET.Element("testsuite", tests=str(tests), failures=str(failures), 
                           errors=str(errors), skipped=str(skipped), disabled=str(disabled))
    
    for key, value in attributes.items():
        testsuite.attrib[key] = value

    if properties:
        props_el = ET.SubElement(testsuite, "properties")
        for key, value in properties.items():
            ET.SubElement(props_el, "property", name=key, value=value)

    for test_case in test_cases:
        testcase_element = ET.SubElement(testsuite, "testcase")
        for key, value in test_case.get('attributes', {}).items():
            testcase_element.attrib[key] = value
        
        if "failure" in test_case:
            failure_element = ET.SubElement(testcase_element, "failure")
            failure_element.text = test_case["failure"]
        
        if "error" in test_case:
            error_element = ET.SubElement(testcase_element, "error")
            error_element.text = test_case["error"]
    
    return testsuite

def create_xunit_tree(suites):
    testsuites = ET.Element("testsuites")
    for ts in suites:
        testsuites.append(create_test_suite(ts))
    logger.debug(f"{ET.tostring(testsuites, 'utf-8')}")
    return testsuites

def get_tc_expected_status(tc):
    if 'failure' in tc: return 'failed'
    return 'passed'

def get_passing_label():
    return 'Test Passed. No Label Required.'

def get_timestamp():
    return str(int((time.time_ns())))

def verify_run_data(want, got):
    assert got["group"] == want["group"]
    assert got["efr"] == want["version"]
    assert got["run_id"] == want["firex_id"]
    assert got["lineup"] == want["lineup"]
    assert got["testsuite_root"] == want["testsuite_root"]

def verify_suite(want, got):
    assert got["timestamp"] == want.get('attributes', {}).get('timestamp')
    assert got["tests"] == len(want.get('test_cases'))
    assert got["failures"] == sum(1 for tc in want.get('test_cases') if "failure" in tc)
    assert got["errors"] == sum(1 for tc in want.get('test_cases') if "error" in tc)
    assert got["disabled"] == sum(1 for tc in want.get('test_cases') if "disabled" in tc)
    assert got["skipped"] == sum(1 for tc in want.get('test_cases') if "skipped" in tc)

    assert len(got["testcases"]) == len(want.get('test_cases'))

    idx = 0
    for i, t in enumerate(got["testcases"]):
        assert t['name'] == want.get('test_cases')[i].get('attributes', {}).get('name', '')
        assert t['time'] == want.get('test_cases')[i].get('attributes', {}).get('time', 0)
        assert t['status'] == get_tc_expected_status(want.get('test_cases')[i])
        idx+=1

class TestCIT:
    def setup_method(self):
        self.next_run_id = 0
        self.runs = []
        self.db = InMemoryDatabase()
        self.db.groups.append('group1')
        self.db.groups.append('group2')
        self.db.labels.append('label1')
        self.db.labels.append('label2')
        self.cit = CIT(self.db)
        
    def teardown_method(self):
        logger.debug(f"data: {self.db.data}")

        for r in self.runs:
            shutil.rmtree(r['run_data']['testsuite_root'])

    def create_run_data(self, run_data={}):
        self.next_run_id+=1

        default_run_data = {
            'version': '24.x.x',
            'workspace': '/nobackup/dummy',
            'framework': 'b4',
            'group': 'group1',
            'chain': 'RunTests',
            'firex_id': f'FireX-Test-000000-000000-00000{self.next_run_id}',
            'lineup': 'xr-dev'
        }
        
        default_run_data.update(run_data)

        testsuite_root = tempfile.mkdtemp()
        default_run_data['testsuite_root'] = testsuite_root

        with open(os.path.join(testsuite_root, 'run.json'), 'w+') as fp:
            json.dump({
                'group': default_run_data['group'],
                'submission_cmd': ['--chain', default_run_data['chain']],
                'firex_id': default_run_data['firex_id'],
                'inputs': {
                    'lineup': default_run_data['lineup'],
                },
            }, fp)

        return default_run_data

    def create_run(self, testsuites, run_data={}):
        run_data = self.create_run_data(run_data)
        for ts in testsuites:
            ts.setdefault("properties", {}).update({
                "testsuite_root": run_data['testsuite_root'],
                "framework": run_data['framework']
            })
            ts.setdefault("attributes", {})
            ts.setdefault("test_cases", [])

        self.runs.append({
            "run_data": run_data,
            "testsuites": testsuites
        })

    def verify_data(self):
        for r in self.runs:
            matching_entries = [e for e in self.db.data if e['run_id'] == r['run_data']['firex_id']]
            assert len(matching_entries) > 0
            for i, e in enumerate(matching_entries):
                verify_run_data(r['run_data'], e)
                verify_suite(r['testsuites'][i], e)

    def test_empty_suite(self):
        self.create_run([
            {
                "attributes": {
                    "timestamp": get_timestamp()
                }
            }
        ])
        
        for r in self.runs:
            root = create_xunit_tree(r['testsuites'])
            self.cit.process_run(root, r['run_data']['version'], r['run_data']['workspace'])

        self.verify_data()
    
    def test_all_pass(self):
        self.create_run([
            {
                "attributes": {
                    "timestamp": get_timestamp()
                },
                "test_cases": [
                    {"attributes": {"name": "test1"}}
                ]
            }
        ])
        
        for r in self.runs:
            root = create_xunit_tree(r['testsuites'])
            self.cit.process_run(root, r['run_data']['version'], r['run_data']['workspace'])

        self.verify_data()


    def test_all_fail(self):
        self.create_run([
            {
                "attributes": {
                    "timestamp": get_timestamp()
                },
                "test_cases": [
                    {"attributes": {"name": "test1"}, "failure": "test failed"}
                ]
            }
        ])
        
        for r in self.runs:
            root = create_xunit_tree(r['testsuites'])
            self.cit.process_run(root, r['run_data']['version'], r['run_data']['workspace'])

        self.verify_data()
                

    @pytest.mark.parametrize("base_plan_id, inherit_plan_id, base_lineup, inherit_lineup, expect_inherit", [
        ("test1", "test1", "xr-dev", "xr-dev", True),
        ("test1", "test2", "xr-dev", "xr-dev", False),
        ("test1", "test1", "xr-dev", "r244x", False),
        ("test1", "test2", "xr-dev", "r244x", False),
    ])
    def test_simple_inheritance(self, base_plan_id, inherit_plan_id, base_lineup, inherit_lineup, expect_inherit):
        tcs = [
            {"attributes": {"name": "test1"}},
            {"attributes": {"name": "test2"}, "failure": "test failed", "label": "label1"},
            {"attributes": {"name": "test3"}, "failure": "test failed", "label": "label2"}
        ]

        self.create_run([
            {
                "attributes": {
                    "timestamp": get_timestamp()
                },
                "properties": {
                    "test.plan_id": base_plan_id
                },
                "test_cases": tcs
            }
        ], run_data={"lineup": base_lineup})

        self.create_run([
            {
                "attributes": {
                    "timestamp": get_timestamp()
                },
                "properties": {
                    "test.plan_id": inherit_plan_id
                },
                "test_cases": tcs
            }
        ], run_data={"lineup": inherit_lineup})
        
        base_run = create_xunit_tree(self.runs[0]['testsuites'])
        self.cit.process_run(base_run, self.runs[0]['run_data']['version'], self.runs[0]['run_data']['workspace'])
        assert self.db.data[0]['bugs'] == []
        for i, tc in enumerate(self.db.data[0]['testcases']):
            if i == 0: assert tc['label'] == get_passing_label()
            else: assert tc['label'] == ''

        ddts = [{'name': 'My Awesome TechZone', 'type': 'TechZone'}]
        self.db.data[0]['bugs'] = ddts
        for i, tc in enumerate(self.db.data[0]['testcases']):
            if i > 0: tc['label'] = tcs[i]['label']

        inherit_run = create_xunit_tree(self.runs[1]['testsuites'])
        self.cit.process_run(inherit_run, self.runs[1]['run_data']['version'], self.runs[1]['run_data']['workspace'])
        
        if(expect_inherit):
            assert len(ddts) == len(self.db.data[1]['bugs'])
            assert ddts[0]['name'] == self.db.data[1]['bugs'][0]['name']
            assert ddts[0]['type'] == self.db.data[1]['bugs'][0]['type']
            for i, tc in enumerate(self.db.data[1]['testcases']):
                if i > 0: assert tc['label'] == tcs[i]['label']
        else:
            assert self.db.data[1]['bugs'] == []
            for i, tc in enumerate(self.db.data[1]['testcases']):
                if i == 0: assert tc['label'] == get_passing_label()
                else: assert tc['label'] == '' or tc['generated']
        
        self.verify_data()

    def test_inheritance_for_pass(self):
        self.create_run([
            {
                "attributes": {
                    "timestamp": get_timestamp()
                },
                "properties": {
                    "test.plan_id": "test1"
                },
                "test_cases": [{"attributes": {"name": "test1"}, "failure": "test failed"}]
            }
        ])
        
        self.create_run([
            {
                "attributes": {
                    "timestamp": get_timestamp()
                },
                "properties": {
                    "test.plan_id": "test1"
                },
                "test_cases": [{"attributes": {"name": "test1"}}]
            }
        ])
        
        base_run = create_xunit_tree(self.runs[0]['testsuites'])
        self.cit.process_run(base_run, self.runs[0]['run_data']['version'], self.runs[0]['run_data']['workspace'])
        assert self.db.data[0]['bugs'] == []

        ddts = [{'name': 'My Awesome TechZone', 'type': 'TechZone'}]
        self.db.data[0]['bugs'] = ddts

        inherit_run = create_xunit_tree(self.runs[1]['testsuites'])
        self.cit.process_run(inherit_run, self.runs[1]['run_data']['version'], self.runs[1]['run_data']['workspace'])
        assert len(ddts) == len(self.db.data[1]['bugs'])
        assert ddts[0]['name'] == self.db.data[1]['bugs'][0]['name']
        assert ddts[0]['type'] == self.db.data[1]['bugs'][0]['type']

        self.verify_data()

    def test_inheritance_after_pass(self):
        self.create_run([
            {
                "attributes": {
                    "timestamp": get_timestamp()
                },
                "properties": {
                    "test.plan_id": "test1"
                },
                "test_cases": [{"attributes": {"name": "test1"}, "failure": "test failed"}]
            }
        ]),
        self.create_run([
            {
                "attributes": {
                    "timestamp": get_timestamp()
                },
                "properties": {
                    "test.plan_id": "test1"
                },
                "test_cases": [{"attributes": {"name": "test1"}}]
            }
        ])
        
        self.create_run([
            {
                "attributes": {
                    "timestamp": get_timestamp()
                },
                "properties": {
                    "test.plan_id": "test1"
                },
                "test_cases": [{"attributes": {"name": "test1"}, "failure": "test failed"}]
            }
        ])
        
        base_run = create_xunit_tree(self.runs[0]['testsuites'])
        self.cit.process_run(base_run, self.runs[0]['run_data']['version'], self.runs[0]['run_data']['workspace'])
        assert self.db.data[0]['bugs'] == []

        ddts = [{'name': 'My Awesome TechZone', 'type': 'TechZone'}]
        self.db.data[0]['bugs'] = ddts

        inherit_run = create_xunit_tree(self.runs[1]['testsuites'])
        self.cit.process_run(inherit_run, self.runs[1]['run_data']['version'], self.runs[1]['run_data']['workspace'])
        assert len(ddts) == len(self.db.data[1]['bugs'])
        assert ddts[0]['name'] == self.db.data[1]['bugs'][0]['name']
        assert ddts[0]['type'] == self.db.data[1]['bugs'][0]['type']

        inherit_run = create_xunit_tree(self.runs[2]['testsuites'])
        self.cit.process_run(inherit_run, self.runs[2]['run_data']['version'], self.runs[2]['run_data']['workspace'])
        assert len(ddts) == len(self.db.data[1]['bugs'])
        assert ddts[0]['name'] == self.db.data[1]['bugs'][0]['name']
        assert ddts[0]['type'] == self.db.data[1]['bugs'][0]['type']

        self.verify_data()

    def test_no_inheritance_after_clear(self):
        self.create_run([
            {
                "attributes": {
                    "timestamp": get_timestamp()
                },
                "properties": {
                    "test.plan_id": "test1"
                },
                "test_cases": [{"attributes": {"name": "test1"}, "failure": "test failed"}]
            }
        ])

        self.create_run([
            {
                "attributes": {
                    "timestamp": get_timestamp()
                },
                "properties": {
                    "test.plan_id": "test1"
                },
                "test_cases": [{"attributes": {"name": "test1"}, "failure": "test failed"}]
            }
        ])

        self.create_run([
            {
                "attributes": {
                    "timestamp": get_timestamp()
                },
                "properties": {
                    "test.plan_id": "test1"
                },
                "test_cases": [{"attributes": {"name": "test1"}, "failure": "test failed"}]
            }
        ])
        
        base_run = create_xunit_tree(self.runs[0]['testsuites'])
        self.cit.process_run(base_run, self.runs[0]['run_data']['version'], self.runs[0]['run_data']['workspace'])
        assert self.db.data[0]['bugs'] == []

        ddts = [{'name': 'My Awesome TechZone', 'type': 'TechZone'}]
        self.db.data[0]['bugs'] = ddts

        inherit_run = create_xunit_tree(self.runs[1]['testsuites'])
        self.cit.process_run(inherit_run, self.runs[1]['run_data']['version'], self.runs[1]['run_data']['workspace'])
        assert len(ddts) == len(self.db.data[1]['bugs'])
        assert ddts[0]['name'] == self.db.data[1]['bugs'][0]['name']
        assert ddts[0]['type'] == self.db.data[1]['bugs'][0]['type']

        self.db.data[1]['bugs'] = []

        no_inherit_run = create_xunit_tree(self.runs[2]['testsuites'])
        self.cit.process_run(no_inherit_run, self.runs[2]['run_data']['version'], self.runs[2]['run_data']['workspace'])
        assert self.db.data[2]['bugs'] == []

        self.verify_data()

    @pytest.mark.parametrize("base_size, inherit_size", [
        (0, 0),
        (0, 1),
        (1, 0),
        (1, 1),
        (1, 2),
        (2, 1),
    ])
    def test_inheritance_mismatch_size(self, base_size, inherit_size):
        def generate_test_cases(n):
            tcs = []
            for i in range(n):
                if i == n - 1:
                    tcs.append({"attributes": {"name": f"test{i+1}"}, "failure": "test failed"})   
                else:
                    tcs.append({"attributes": {"name": f"test{i+1}"}})
            return tcs
        
        self.create_run([
            {
                "attributes": {
                    "timestamp": get_timestamp()
                },
                "properties": {
                    "test.plan_id": "test1"
                },
                "test_cases": generate_test_cases(base_size)
            }
        ])

        self.create_run([
            {
                "attributes": {
                    "timestamp": get_timestamp()
                },
                "properties": {
                    "test.plan_id": "test1"
                },
                "test_cases": generate_test_cases(inherit_size)
            }
        ])
        
        base_run = create_xunit_tree(self.runs[0]['testsuites'])
        self.cit.process_run(base_run, self.runs[0]['run_data']['version'], self.runs[0]['run_data']['workspace'])
        assert self.db.data[0]['bugs'] == []

        ddts = [{'name': 'My Awesome TechZone', 'type': 'TechZone'}]
        self.db.data[0]['bugs'] = ddts

        inherit_run = create_xunit_tree(self.runs[1]['testsuites'])
        self.cit.process_run(inherit_run, self.runs[1]['run_data']['version'], self.runs[1]['run_data']['workspace'])
        
        assert len(ddts) == len(self.db.data[1]['bugs'])
        assert ddts[0]['name'] == self.db.data[1]['bugs'][0]['name']
        assert ddts[0]['type'] == self.db.data[1]['bugs'][0]['type']

        self.verify_data()