import xml.etree.ElementTree as ET
import argparse
import os

def _fix_testsuite_class(ts):
    for t in ts.findall('testcase'):
        if t.get('name') == '_':
            ts.remove(t)
            continue
        cls = ts.get('name') + '.' + t.get('name').split('/')[0]
        t.set('classname', cls)

def _find_ts_name(ts):
    for p in ts.findall('./properties/property'):
        if p.get('name') == 'test.plan_id':
            return p.get('value').replace('.', '-')
    return ''

def _rewrite(file, outfile):
    tree = ET.parse(file)

    for ts in tree.getroot().findall('./testsuite'):
        if not ts.get('name', default=''):
            ts.set('name', _find_ts_name(ts))
        _fix_testsuite_class(ts)
    tree.write(outfile)

def _is_valid_file(parser, arg):
    if not os.path.exists(arg):
        parser.error("File %s does not exist" % arg)
    else:
        return arg

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Format xunit test result file')
    parser.add_argument('file', type=lambda x: _is_valid_file(parser, x),
                        help='xunit result file')
    parser.add_argument('outfile', help='output file')
    args = parser.parse_args()
    _rewrite(args.file, args.outfile)
