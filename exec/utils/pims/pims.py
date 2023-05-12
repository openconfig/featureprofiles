import argparse
import json
import os

from subprocess import check_output
from operator import itemgetter

parser = argparse.ArgumentParser(description='Find latest pims image')
parser.add_argument('--lineup', default='xr-dev', help="image lineup")
parser.add_argument('--label', default='*NIGHTLY*', help="image label")
args = parser.parse_args()

lineup = args.lineup
nightly_label = args.label
image_subpaths = ['8000/8000-x64.iso', 'img-8000/8000-x64.iso']

candidates = []
pims_output = check_output([
    '/usr/cisco/bin/pims',
    'lu',
    'released_efrs',
    '-lineup',
    lineup,
    '-label',
    nightly_label
], encoding='utf-8')

for l in pims_output.splitlines():
    if l.startswith('EFR-'):
        parts = l.split('\t')
        labels = parts[-1].split(',')
        for lb in labels:
            candidates.append((lb, parts[0]))

candidates = sorted(candidates,key=itemgetter(0), reverse=True)
image_path = None
image_label = None
image_efr = None

for c in candidates:
    label, efr = c
    js = json.loads(check_output([
        '/usr/cisco/bin/pims',
        'gsr',
        '-r',
        'nightly_build_info',
        '-nightly_label',
        label,
        '-lineup',
        lineup,
        '-format',
        'json'
    ], encoding='utf-8'))
    
    for result in js:
        if result.get('Status') == 'Successful':
            image_dir = result.get('Image Location')
            for subpath in image_subpaths:
                location = os.path.join(image_dir, subpath)
                if os.path.exists(location):
                    image_path = location
                    image_label = label
                    image_efr = efr
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
