import xml.etree.ElementTree as ET
import argparse
import os

def _fix_testsuite_class(ts):
    current_class = ''

    for t in ts.findall('testcase'):
        if not current_class:
            current_class = t.get('name')

        t.set('classname', current_class)

        if not t.get('name').startswith(current_class):
            current_prefix = ''
            current_class = ''

def _find_ts_name(ts):
    for p in ts.findall('./properties/property'):
        if p.get('name') == 'testsuite_name':
            return p.get('value')
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
    parser.add_argument('outfile', type=lambda x: _is_valid_file(parser, x),
                        help='output file')
    args = parser.parse_args()
    _rewrite(args.file, args.outfile)