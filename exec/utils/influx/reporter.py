from influxdb_client import InfluxDBClient, Point
from influxdb_client.client.write_api import SYNCHRONOUS, PointSettings
import xml.etree.ElementTree as ET
import argparse
import os

INFLUX_URL = "http://webdt-ci-lnx:8086"
INFLUX_TOKEN = "5adUTkY696W7e3m1wf9tH_0rcTGELhHX7dlpWBK00pfHNd0R5k50P_TQrNVjyF-x61dBataaqQy-fvh0IIXVEA=="
INFLUX_ORG = "webdt"
INFLUX_BUCKET = "firex"

# class FireXTest:
#   class Status:
#     PASS = 1
#     FAILED = 2
#     SKIPPED = 3
#     ABORTED = 4
    
#   def __init__(self, xml_test):
#     self._id = xml_test.get('name')
#     self._runtime = float(xml_test.get('time', default=0))
#     self._status = FireXTest.Status.PASS
#     self._logs = ''

#     for e in xml_test:
#       if e.tag == 'failure':
#         self._status = FireXTest.Status.FAILED
#         self._logs = e.text
#         break
#       if e.tag == 'error':
#         self._status = FireXTest.Status.ABORTED
#         break
#       if e.tag in ['disabled', 'skipped']:
#         self._status = FireXTest.Status.SKIPPED
#         self._logs = e.text
#         break
    
# class FireXSuite:    
#   def __init__(self, ts):
#     self._total = int(ts.get('tests', default=0))
#     self._failures = int(ts.get('failures', default=0))
#     self._errors = int(ts.get('errors', default=0))
#     self._skipped = int(ts.get('skipped', default=0))
#     self._disabled = int(ts.get('disabled', default=0))
#     self._runtime = float(ts.get('time', default=0))
#     # self._tests = [FireXTest(t) for t in ts.findall('./testcase')]
    
#     self._id = ts.get('name', default='')
#     if not self._id:
#       properties = ts.findall('./properties/property')
#       self._id = self._find_ts_name(properties)

#   def _did_pass(self):
#     return self._failures == 0 and self._errors == 0
  
#   def _num_executed(self):
#     return self._tests - (self._skipped + self._disabled)

#   # Ondatra specific 
#   def _find_ts_name(self, properties):
#     for np in ['test.plan_id', 'test.path']:
#       for p in properties:
#         if p.get('name') == np:
#           return p.get('value')
#     return ''
    
#   def _to_influx_point(self):
#     return Point("test_results") \
#         .tag("name", self._id) \
#         .field("runtime", self._runtime) \
#         .field("total", self._total) \
#         .field("failures", self._failures) \
#         .field("errors", self._errors) \
#         .field("skipped", self._skipped) \
#         .field("disabled", self._disabled)

# class FireXRun:
#   def __init__(self, id, tree, timestamp=time.time()):
#     self._id = id
#     self._timestamp = timestamp
#     self._suites = [FireXSuite(ts) for ts in tree.getroot().findall('./testsuite')]
  
#  def _to_influx_points(self):
#    return [ts._to_influx_point() for ts in self._suites]

def _fatal(s):
  print(f"[FATAL] {s}")
  exit(1)

def _warn(s):
  print(f"[WARN] {s}")

def _must_get_prop_val(props, name):
  for p in props:
      if p.get('name', default='') == name:
        value = p.get('value', default='')
        if value: return value
  return ''

def _get_ts_name(props):
  # test.plan_id is ondatra specific
  for n in ['test.plan_id', 'testsuite_name']:
    name = _must_get_prop_val(props, n)
    if name: return name
  return ''

def _get_ts_logs(props):
  path_props = ['testsuite_root', 'log']
  path_elems = []
  for n in path_props:
    path_elems.append(_must_get_prop_val(props, n))

  log_file = os.path.join(*path_elems)
  if os.path.isfile(log_file):
    return log_file
  return ''

parser = argparse.ArgumentParser(description='Inject FireX run results in influx')
parser.add_argument('run_id', help="FireX run id")
parser.add_argument('xunit_file', help="xUnit result file")
parser.add_argument('--lineup', default='', help="Image lineup")
parser.add_argument('--efr', default='', help="Image EFR")
parser.add_argument('--group', default='', help="Reporting group")
args = parser.parse_args()

try:
  tree = ET.parse(args.xunit_file)
except Exception as e:
  _fatal(f"error parsing file {args.xunit_file}: {e}")

points = []
for ts in tree.getroot().findall('./testsuite'):
    props = ts.findall('./properties/property')
    name = _get_ts_name(props)
    logs = _get_ts_logs(props)
    url = _must_get_prop_val(props, 'test_details_url')

    total = int(ts.get('tests', default=0))
    failures = int(ts.get('failures', default=0))
    errors = int(ts.get('errors', default=0))
    skipped = int(ts.get('skipped', default=0))
    disabled = int(ts.get('disabled', default=0))
    runtime = float(ts.get('time', default=0))

    if not name:
      _warn("skipping testsuite with no name in input")
      continue
    
    p = Point("test_results") \
        .tag("name", name) \
        .field("runtime", runtime) \
        .field("total", total) \
        .field("failures", failures) \
        .field("errors", errors) \
        .field("skipped", skipped) \
        .field("disabled", disabled) \
        .field("url", url) \
        .field("logs", logs)
    
    # Ondatra specific
    ondatra_props = ['test.uuid', 'test.path', 'test.description',
                     'git.origin', 'git.commit', 'git.commit_timestamp', 
                     'dut.os_version', 'dut.model', 'dut.model.full']
    for op in ondatra_props:
      p = p.field(op, _must_get_prop_val(props, op))

    points.append(p)

point_settings = PointSettings()
point_settings.add_default_tag("run_id", args.run_id)
point_settings.add_default_tag("lineup", args.lineup)
point_settings.add_default_tag("efr", args.efr)
point_settings.add_default_tag("group", args.group)

with InfluxDBClient(url=INFLUX_URL, 
                    token=INFLUX_TOKEN, 
                    org=INFLUX_ORG) as influxc:
  with influxc.write_api(write_options=SYNCHRONOUS, 
                         point_settings=point_settings) as influxw:
    influxw.write(INFLUX_BUCKET, INFLUX_ORG, points)