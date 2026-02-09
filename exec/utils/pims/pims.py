import argparse
import json
import os

from subprocess import check_output
from operator import itemgetter

parser = argparse.ArgumentParser(description='Find latest pims image')
parser.add_argument('--lineup', default='xr-dev', help="image lineup")
parser.add_argument('--label', default='*NIGHTLY*', help="image label")
parser.add_argument('--from_date', default='', help="image from date, defaults to 1 week")
parser.add_argument('--dev', dest='dev', action='store_true')
args = parser.parse_args()

lineup = args.lineup
nightly_label = args.label
from_date = args.from_date
use_dev_image = args.dev
image_subpaths = ['8000/8000-x64.iso', 'img-8000/8000-x64.iso']
dev_image_subpaths = ['8000/8000-dev-x64.iso', 'img-8000/8000-dev-x64.iso']

candidates = []
pims_cmd = [
    '/usr/cisco/bin/pims',
    'lu',
    'released_efrs',
    '-lineup',
    lineup,
    '-label',
    nightly_label
]

if from_date:
    pims_cmd.extend(['-from_date', from_date])

pims_output = check_output(pims_cmd, encoding='utf-8')

image_label = None
image_efr = None

for l in pims_output.splitlines():
    if l.startswith('EFR-'):
        parts = l.split('\t')
        image_efr = parts[0]
        labels = parts[-1].split(',')
        for lb in labels:
            candidates.append((lb, parts[0]))
            image_label = lb

candidates = sorted(candidates,key=itemgetter(0), reverse=True)
image_path = None

for c in candidates:
    label, efr = c
    js = json.loads(check_output([
        '/usr/cisco/bin/pims',
        'gsr',
        '-r',
        'build_status_image_details',
        '-build_label',
        label,
        '-format',
        'json'
    ], encoding='utf-8'))
    
    for result in js:
        if result.get('Build Status') == 'Successful' and result.get('Image Name') == '8000-x64.iso':
            image_ws = result.get('Workspace Location')
            subpaths = image_subpaths
            if use_dev_image: subpaths = dev_image_subpaths
            for subpath in subpaths:
                location = os.path.join(image_ws, subpath)
                if os.path.exists(location):
                    image_path = location
                    break
            if image_path:
                break
    if image_path:
        break

if image_path:
    image_version = check_output(
            f"isoinfo -i {image_path} -x '/MDATA/BUILD_IN.TXT;1' " \
                f"| tail -n1 | cut -d'=' -f2 | cut -d'-' -f1", 
            shell=True,
            encoding='utf-8'
        ).strip()

    print(f'{image_path},{image_version},{image_label},{image_efr}')
