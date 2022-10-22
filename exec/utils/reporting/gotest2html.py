import argparse
import json
import os

def _to_md_anchor(s):
    return f'[{s}](#{s.lower().replace(" ", "-").replace("(", "").replace(")","")})'

class GoTest:
    def __init__(self, name, pkg = None, parent = None):
        self._qname = name
        self._name = name
        self._pkg = pkg
        self._parent = parent
        self._children = []
        self._output = ''
        self._status = ''

        if parent and parent.get_parent():
            self._name = self._qname[len(parent.get_qualified_name()):]

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

    def get_descendants(self):
        desc = self._children.copy()
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

    def get_parent(self):
        return self._parent

    def mark_passed(self):
        self._status = 'Pass'

    def mark_failed(self):
        self._status = 'Fail'

    def mark_skipped(self):
        self._status = 'Skip'

    def did_pass(self):
        return self._status == 'Pass' or self._status == 'Skip'

    def did_skip(self):
        return self._status == 'Skip'

    def get_status(self):
        return self._status

    def _pass_text(self):
        if len(self.get_descendants()) == 0:
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
        md = ('&nbsp;&nbsp;&nbsp;&nbsp;' * level) + ('*' * level) + em + name + em + ' | ' + self._pass_text() + '\n'
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
$(function () {
    var data = String.raw`"""+table_data.replace("`", "'")+"""`
    var summary_data = String.raw`""" + summary_data.replace("`", "'") + """`

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
                title: "Pass",
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

def _parse(file, json_data):
    test_map = {}
    top_test = GoTest(os.path.basename(file.split('.')[0]), file)

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
    summary = {"total": 0, "passed": 0, "failed": 0}
    for f in files:
        content = _read_log_file(f)
        test = _parse(f, json.loads(content))
        summary["total"] += len(test.get_descendants())
        summary["skipped"] += len(test.get_skipped_descendants())
        summary["passed"] += len(test.get_passed_descendants())
        data.append(test.to_table_data())
    summary["failed"] = summary["total"] - summary["passed"]
    summary["passed"] = summary["passed"] - summary["skipped"]
    return _generate_html(json.dumps(data), json.dumps(summary))


def to_markdown(files):
    details_md = "## Tests\n"
    suite_summary = []

    for f in files:
        content = _read_log_file(f)
        test = _parse(f, json.loads(content))
        total = len(test.get_descendants())
        skiped = len(test.get_skipped_descendants())
        passed = len(test.get_passed_descendants())
        failed = total - passed
        passed -= skiped

        suite_summary.append({
            "suite": test.get_qualified_name(),
            "total": total, 
            "passed": passed, 
            "failed": failed,
            "skipped": skiped
        })

        details_md += "### " + test.get_qualified_name()+ "\n"
        details_md +=  "Test | Pass\n"
        details_md += "-----|------\n"

        for t in test._children:
            details_md += t.to_md_string()

        for t in test._children:
            details_md += "#### " + t.get_qualified_name()+ "\n"
            details_md +=  "Test | Pass\n"
            details_md += "-----|------\n"
            details_md += t.to_md_string(recursive=True)
    
    suite_summary_md = "## Summary\n"
    suite_summary_md += f"""
Suite | Total | Passed | Failed | Skipped
------|-------|--------|--------|--------
"""
    for s in suite_summary:
        suite_summary_md += f'{_to_md_anchor(s["suite"])} | {s["total"]} | {s["passed"]} | {s["failed"]} | {s["skipped"]}\n'

    return suite_summary_md + details_md

def _is_valid_file(parser, arg):
    if not os.path.exists(arg):
        parser.error("File %s does not exist" % arg)
    else:
        return arg

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