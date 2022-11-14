import argparse
import pathlib
import hashlib
import urllib
import constants
import json
import os
import re

from datetime import datetime
from ddts_utils import get_ddts_info
from gh_utils import FPGHRepo

def _to_md_anchor(s):
    sanitized = re.sub('[^0-9a-zA-Z_\-\s]+', '', s).strip().replace(' ', '-').lower()
    return f'[{s}](#{sanitized})'

class GoTestSuite:
    def __init__(self, run_id = '', last_updated = datetime.now().timestamp()):
        self._tests = []
        self._last_run_id = run_id
        self._last_updated = last_updated

    @staticmethod
    def from_json_obj(obj):
        ts = GoTestSuite(run_id = obj['last_run_id'], last_updated=obj['last_updated'])
        ts._tests = [GoTest.from_json_obj(c) for c in obj['tests']]
        return ts

    def update(self, run_id, go_tests, last_updated = datetime.now().timestamp()):
        self._last_run_id = run_id
        self._last_updated = last_updated
        self._tests = go_tests

    def add_test(self, go_test):
        self._tests.append(go_test)

    def get_tests(self, recursive=False):
        if not recursive:
            return self._tests
        tests = [t for t in self._tests]
        for t in self._tests:
            tests.extend(t.get_descendants())
        return tests

    def get_last_updated(self):
        return self._last_updated

    def get_last_run_id(self):
        return self._last_run_id

    def get_stats(self):
        stats = {
            'total': 0,
            'passed': 0,
            'failed': 0,
            'skipped': 0,
            'regressed': 0,
        }
        for t in self.get_tests():
            stats['total'] += t.get_total()
            stats['passed'] += t.get_total_passed()
            stats['failed'] += t.get_total_failed()
            stats['skipped'] += t.get_total_skipped()
            stats['regressed'] += t.get_total_regressed()
        return stats

    def to_json_obj(self):
        return {
            'last_run_id': self.get_last_run_id(),
            'last_updated': self.get_last_updated(),
            'tests': [t.to_json_obj() for t in self._tests]
        }

    def to_html(self):
        data = [ ]
        summary = {"total": 0, "passed": 0, "failed": 0, "skipped": 0}
        for test in self.get_tests():
            summary["total"] += test.get_total()
            summary["skipped"] += test.get_total_skipped()
            summary["passed"] += test.get_total_passed()
            summary["failed"] += test.get_total_failed()
            data.append(test.to_table_data())
        return _generate_html(json.dumps(data), json.dumps([summary]))

    def to_md_string(self):
        details_md = "## Tests\n"
        suite_summary = []

        for test in self._tests:
            suite_summary.append({
                "test": test,
                "suite": test.get_qualified_name(),
                "total": test.get_total(), 
                "passed": test.get_total_passed(), 
                "failed": test.get_total_failed(),
                "skipped": test.get_total_skipped(),
                "regressed": test.get_total_regressed()
            })

            details_md += "### " + test.get_qualified_name()+ "\n"
            details_md +=  "Test | Logs | Result \n"
            details_md += "------|------|------\n"

            for t in test._children:
                details_md += t.to_md_string()

            for t in test._children:
                details_md += "#### " + t.get_qualified_name()+ "\n"
                details_md +=  "Test | Logs | Result \n"
                details_md += "------|------|------\n"
                details_md += t.to_md_string(recursive=True)
        
        suite_summary_md = "## Test Suites\n"
        suite_summary_md += f"""
Suite | T | P | F | S | Logs | DDTS | Attr | Result
------|---|---|---|---|------|------|------|-------
"""

        total, passed, failed, skipped, regressed = [0] * 5
        for s in suite_summary:
            total += s["total"]
            passed += s["passed"]
            failed += s["failed"]
            skipped += s["skipped"]
            regressed += s["regressed"]

            result = ''
            if s['regressed'] > 0: result = ':warning:'
            elif s['failed'] > 0: result = ':x:'
            elif s['total'] > 0: result = ':white_check_mark:'

            log_url_parts = s["test"].get_logs_url().split("/")
            html_logs_url = "/".join(log_url_parts[0:-1] + [urllib.parse.quote(log_url_parts[-1].replace(".json", ".html"), safe="")])
            raw_logs_url = "/".join(log_url_parts[0:-1] + ['output_from_json.log'])
            testbed_logs_url = "/".join(log_url_parts[0:-1] + ['show_version.txt'])

            gh_issue = s["test"].get_gh_issue()
            title = _to_md_anchor(s["suite"])

            ddts = ''
            if gh_issue: 
                title += f' [(#{gh_issue["number"]})]({gh_issue["link"]})'
                for bug in gh_issue["bugs"]:
                    bug_data = get_ddts_info(bug["label"])
                    ddts += f'[{bug_data["custom_label"]}]({bug["link"]})<br />'

            result_attr = []
            if s["test"].is_patched(): result_attr.append('P')
            if s["test"].is_deviated(): result_attr.append('D')
            if len(result_attr) > 0:
                result_attr = f'{", ".join(result_attr)}'
            else: result_attr = ''

            suite_summary_md += f'{title} | {s["total"]} | {s["passed"]} | {s["failed"]} | {s["skipped"]}'
            suite_summary_md += f'| [HTML]({html_logs_url}) [RAW]({raw_logs_url}) [Testbed]({testbed_logs_url})'
            suite_summary_md += f'| {ddts} | {result_attr} | {result}\n'

        return f"""
## Summary
Total (T) | Passed (P) | Failed (F) | Skipped (S)
----------|------------|------------|------------
{total}|{passed}|{failed}|{regressed}|{skipped}
""" + suite_summary_md + details_md

class GoTest:
    def __init__(self, name, pkg = None, parent = None):
        self._qname = name
        self._name = name
        self._pkg = pkg
        self._parent = parent
        self._children = []
        self._output = ''
        self._status = ''
        self._failures = []
        self._gh_issue = None
        self._deviated = False
        self._patched = False
        self._log_file_name = hashlib.md5(name.encode("utf")).hexdigest() + '.txt'

        if parent:
            self._logs_url = os.path.join(constants.gh_logs_dir, self._log_file_name)
            if parent.get_parent():
                self._name = self._qname[len(parent.get_qualified_name()):]
        else:
            self._logs_url = pkg.replace(constants.base_logs_dir, constants.base_logs_url)


    @staticmethod
    def from_json_obj(obj, parent = None):
        gt = GoTest(obj['name'], obj['pkg'], parent)
        gt._qname = obj['qname']
        gt._status = obj['status']
        gt._children = [GoTest.from_json_obj(c, parent) for c in obj['children']]
        return gt

    def append_output(self, str):
        self._output += str

    def create_child(self, name, pkg):
        self._children.append(GoTest(name, pkg, self))
        return self._children[-1]

    def get_name(self):
        return self._name

    def get_qualified_name(self):
        return self._qname

    def get_package(self):
        return self._pkg

    def get_output(self):
        return self._output

    def get_children(self):
        return self._children

    def get_descendants(self):
        desc = [c for c in self._children]
        for c in self._children:
            desc.extend(c.get_descendants())
        return desc

    def get_passed_descendants(self):
        desc = [c for c in self._children if c.did_pass()]
        for c in self._children:
            desc.extend(c.get_passed_descendants())
        return desc

    def get_skipped_descendants(self):
        desc = [c for c in self._children if c.did_skip()]
        for c in self._children:
            desc.extend(c.get_skipped_descendants())
        return desc

    def get_regressed_descendants(self):
        desc = [c for c in self._children if c.did_regress()]
        for c in self._children:
            desc.extend(c.get_regressed_descendants())
        return desc

    def get_parent(self):
        return self._parent

    def get_logs_url(self):
        return self._logs_url

    def get_log_file_name(self):
        return self._log_file_name

    def get_gh_issue(self):
        return self._gh_issue

    def set_gh_issue(self, gh_issue):
        self._gh_issue = gh_issue

    def mark_passed(self):
        self._status = 'Pass'

    def mark_failed(self):
        self._status = 'Fail'

    def mark_skipped(self):
        self._status = 'Skip'
        
    def mark_patched(self):
        self._patched = True

    def mark_deviated(self):
        self._deviated = True
        
    def add_failure(self, failure):
        self._failures.append(failure)

    def find_failures(self, known_failures):
        for wf in known_failures:
            # default desc
            msg = '${name}'
            if "ddts" in wf: msg += ' ${ddts}'

            for m in wf["match"]:
                if 'message' in m:
                    msg = m['message']
                
                msg = msg.replace('${name}', wf["name"])
                if "ddts" in wf:
                    msg = msg.replace('${ddts}', f'[({wf["ddts"]})]({constants.base_bug_tracker_url}{wf["ddts"]})')
                
                if "string" in m and m["string"] in self._output:
                    self.add_failure(msg)

                elif "pattern" in m:
                    match = re.findall(m["pattern"], self._output)
                    for m in match:
                        msg_cpy = msg
                        for idx in re.findall('\${match\[(\d*)\]}', msg_cpy):
                            element = m
                            if type(m) is tuple:
                                element = m[int(idx)]
                            msg_cpy = msg_cpy.replace('${match['+idx+']}', element)
                        self.add_failure(msg_cpy)

    def did_pass(self):
        return self._status == 'Pass' or self._status == 'Skip'

    def did_skip(self):
        return self._status == 'Skip'

    def did_fail(self):
        return  self._status == 'Fail'

    def did_regress(self):
        if not self._gh_issue:
            return False
        return self.did_fail() and 'Pass' in self._gh_issue.tags

    def update_gh_issue(self):
        if not self._gh_issue:
            return

        did_pass = True
        for c in self.get_children():
            if not c.did_pass():
                did_pass = False

        tags = []
        if did_pass: tags.append('auto: pass')
        else: tags.append('auto: fail')
        if self.is_deviated(): tags.append('auto: deviation')
        if self.is_patched(): tags.append('auto: patched')
        if not did_pass and 'Pass' in self._gh_issue['tags']: tags.append('auto: regression')
        FPGHRepo.instance().update_labels(self._gh_issue['number'], tags)
        
    def get_status(self):
        return self._status

    def is_patched(self):
        return self._patched
    
    def is_deviated(self):
        return self._deviated

    def get_total(self):
        return len(self.get_descendants())
    
    def get_total_skipped(self):
        return len(self.get_skipped_descendants())

    def get_total_regressed(self):
        return len(self.get_regressed_descendants())

    def get_total_passed(self):
        return len(self.get_passed_descendants()) - self.get_total_skipped()
    
    def get_total_failed(self):
        return self.get_total() - self.get_total_passed() - self.get_total_skipped()

    def _pass_text(self):
        if len(self.get_descendants()) == 0:
            if len(self._failures) > 0:
                return '<br />'.join(self._failures)
            return self._status
        elif len(self.get_passed_descendants()) != len(self.get_descendants()):
            return str(len(self.get_passed_descendants())) + "/" + str(len(self.get_descendants()))
        else:
            return 'Pass'

    def _status_text(self):
        if self._parent:
            return self._status
        if len(self.get_passed_descendants()) == len(self.get_descendants()):
            return 'Pass'
        return 'Fail'

    def to_json_obj(self):
        return {
            'qname': self._qname,
            'name': self._name,
            'pkg': self._pkg,
            'status': self._status,
            'children': [c.to_json_obj() for c in self._children],
        }
        
    def to_table_data(self):
        return {
            "name": self.get_qualified_name(),
            "output": self.get_output(),
            "status": self._status_text(),
            "pass": self._pass_text(),
            "_children": [c.to_table_data() for c in self._children]
        }
        
    def to_md_string(self, recursive = False, level = 0):
        em = ''
        if level == 0: em = '**'
        name = self.get_name()
        if not recursive and level == 0: 
            name = _to_md_anchor(self.get_name())
        md = ('&nbsp;&nbsp;&nbsp;&nbsp;' * level) + ('*' * level) + em + name + em 
        md += f' | [Logs]({self.get_logs_url()}) | ' + self._pass_text() + '\n'
        if recursive:
            for c in self._children:
                md += c.to_md_string(recursive, level+1)
        return md

def _generate_html(table_data, summary_data):
    return """
 <!DOCTYPE html>
<html>
<head>
<link href="https://unpkg.com/tabulator-tables/dist/css/tabulator.min.css" rel="stylesheet"/>
<link rel="stylesheet" href="https://code.jquery.com/ui/1.12.1/themes/smoothness/jquery-ui.css"/>
<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.1.1/css/all.min.css"/>
<script src="https://ajax.googleapis.com/ajax/libs/jquery/3.6.0/jquery.min.js"></script>
<script src="https://code.jquery.com/jquery-3.6.0.js"></script>
<script src="https://code.jquery.com/ui/1.13.1/jquery-ui.js"></script>
<script type="text/javascript" src="https://unpkg.com/tabulator-tables/dist/js/tabulator.min.js"></script>
<style>
.tabulator .tabulator-row.tabulator-selectable:hover {
  background-color: #bbb;
  cursor: pointer;
}

#summary {
  margin-bottom:10px;
}
#summary .tabulator-row {
  font-weight: bold;
  text-align: center;
}
</style>
</head>
<body>
<div id="summary"></div>
<div id="table"></div>
<script>

function findTestByName(data, target) {
    if(data == null) {
        return null;
    }
    if(Array.isArray(data)) {
        for(var i=0; i<data.length; i++) {
            test = findTestByName(data[i], target);
            if(test) return test;
        }
    }
    else {
        if(data.name == target) {
            return data
        } else if(data._children) {
            return findTestByName(data._children, target)
        }
    }
    return null;
}

function initTables(data, summary_data) {
    new Tabulator("#summary", {
        layout: "fitColumns",
        data: summary_data,
        columns: [
            {
            title: "Summary",
            headerHozAlign: "center",
            columns: [
                {
                    title: "Total",
                    field: "total",
                },
                {
                    title: "Passed",
                    field: "passed",
                },
                {
                    title: "Failed",
                    field: "failed",
                    formatter: function (cell, formatterParams, onRendered) {
                        cell.getElement().style.color = "#990000";
                        return cell.getValue();
                    }
                },
                {
                    title: "Skipped",
                    field: "skipped",
                }
            ]
            }
        ]
    })

    new Tabulator("#table", {
        height: "100%",
        layout: "fitColumns",
        data: data,
        dataTree: true,
        dataTreeStartExpanded:function(row, level){
            return row.getData().status == "Fail";
        },
        dataTreeSelectPropagate: true,
        columns: [
            {
                title: "Test",
                field: "name",
                formatter: function (cell, formatterParams, onRendered) {
                    v = cell.getValue();
                    if (!cell.getRow().getTreeParent())
                        return "<b>" + v + "</b>"
                    return v;
                }
            },
            {
                title: "Logs",
                formatter: function(cell, formatterParams, onRendered) {
                    output = cell.getRow().getData().output;
                    if(output && output.length > 0)
                        return "<i class='fa-solid fa-file-lines'></i>";
                    return "";
                },
                cellClick: function(e, cell) {
                    output = cell.getRow().getData().output;
                    var popup = window.open('', "", "width=800, height=1024, scrollbars=yes");
                    $(popup.document.body).html("<pre>" + output + "</pre>");
                },
                hozAlign: "center",
                width: 100
            },
            {
                title: "Result",
                field: "pass",
                formatter: function (cell, formatterParams, onRendered) {
                    v = cell.getValue();
                    row = cell.getRow();
                    status = row.getData()["status"];
                    if(status == "Fail" && row.getTreeChildren().length == 0) {
                        row.getElement().style.color = "#990000";
                        row.getElement().style.fontWeight = "900";
                    } 
                    cell.getElement().style.fontWeight = "900";
                    return v;
                },
                hozAlign: "center",
                width: 100
            }
        ]
    });
}

$(function () {
    var data = String.raw`"""+table_data.replace("`", "'")+"""`
    var summary_data = String.raw`"""+summary_data+"""`

    var queryDict = {}
    location.search.substr(1).split("&").forEach(function(item) {
        queryDict[item.split("=")[0]] = decodeURIComponent(item.split("=")[1])
    })

    if('output' in queryDict) {
        testName = queryDict['output'];
        test = findTestByName(JSON.parse(data), testName)
        if(test && test.output && test.output.length > 0)
            $("#summary").html("<pre>" + test.output + "</pre>");
        else
            initTables(data, summary_data);
    } else {
        initTables(data, summary_data);
    }
});
</script>
</body>
</html>   
"""

def _get_parent(test_map, entry, default):
    candidates = [test_map[t] for t in test_map if t[1] == entry["Package"]]
    candidates.sort(key=lambda t: len(t.get_qualified_name()), reverse=True)
    for c in candidates:
        if entry["Test"].startswith(c.get_qualified_name() + '/'):
            return c
    return default

def _parse(file, json_data, suite_name=None, known_failures=[]):
    test_map = {}
    if not suite_name: suite_name = pathlib.Path(file).stem
    top_test = GoTest(suite_name, file)

    for entry in json_data:
        if 'Test' not in entry:
            if entry["Action"] == "fail":
                pkg_tests = [test_map[t] for t in test_map if t[1] == entry["Package"]]
                for t in pkg_tests:
                    if not t.get_status():
                        t.mark_failed()
            continue

        test_name = entry['Test']
        test_pkg = entry['Package']

        if entry["Action"] == 'run':
            c = _get_parent(test_map, entry, top_test).create_child(test_name, test_pkg)
            test_map[(test_name, test_pkg)] = c
                
        elif entry["Action"] == 'output':
            test_map[(test_name, test_pkg)].append_output(entry["Output"])

        elif entry["Action"] == 'pass':
            test_map[(test_name, test_pkg)].mark_passed()

        elif entry["Action"] == 'fail':
            test_map[(test_name, test_pkg)].mark_failed()
        
        elif entry["Action"] == 'skip':
            test_map[(test_name, test_pkg)].mark_skipped()

    for t in test_map.values():
        t.find_failures(known_failures)

    return top_test

def _read_log_file(file):
    content = '['
    with open(file, 'r') as fp:
        lines = fp.readlines()
        for i, f in enumerate(lines):
            content += f
            if i < len(lines) - 1: content += ','
    content += ']'
    return content

def to_html(files):
    data = [ ]
    summary = {"total": 0, "passed": 0, "failed": 0, "skipped": 0}
    for f in files:
        try:
            content = _read_log_file(f)
            test = _parse(f, json.loads(content))
        except: continue

        summary["total"] += test.get_total()
        summary["skipped"] += test.get_total_skipped()
        summary["passed"] += test.get_total_passed()
        summary["failed"] += test.get_total_failed()
        data.append(test.to_table_data())
    return _generate_html(json.dumps(data), json.dumps([summary]))


def to_markdown(files):
    test_suite = GoTestSuite()
    for f in files:
        try:
            content = _read_log_file(f)
            test = _parse(f, json.loads(content))
            test_suite.add_test(test)
        except: continue
    return test_suite.to_md_string()

def parse_json(file, suite_name=None, known_failures=[]):
    return _parse(file, json.loads(_read_log_file(file)), suite_name=suite_name, known_failures=known_failures)

def _is_valid_file(parser, arg):
    if not os.path.exists(arg):
        parser.error("File %s does not exist" % arg)
    else:
        return arg

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Generate HTML report from go test logs')
    parser.add_argument('files', metavar='FILE', nargs='+',
                        type=lambda x: _is_valid_file(parser, x),
                        help='go test log files in json format')
    parser.add_argument('--md', default=False, action='store_true', help="generate md report")
    args = parser.parse_args()

    if args.md:
        print(to_markdown(args.files))
    else:
        print(to_html(args.files))