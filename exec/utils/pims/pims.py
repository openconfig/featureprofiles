from collections import namedtuple
import json
import os
from subprocess import check_output

lineup = 'xr-dev'
nightly_label = '*NIGHTLY*'
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
        candidates.extend(labels)

candidates.sort(reverse=True)

image_path = None
image_label = None

for label in candidates:
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
                    break
            if image_path:
                break
    if image_path:
        break

if image_path:
    image_version = check_output(
            f"/usr/bin/isoinfo -i {image_path} -x '/MDATA/BUILD_IN.TXT;1' " \
                f"| tail -n1 | cut -d'=' -f2 | cut -d'-' -f1", 
            shell=True,
            encoding='utf-8'
        ).strip()

    print(f'{image_path},{image_version},{image_label}')