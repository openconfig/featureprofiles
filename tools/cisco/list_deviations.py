from pathlib import Path
import re

rx = r'platform:\s{(.|\n)*?vendor:\sCISCO(.|\n)*?}(\s|\n)*deviations:\s{((.|\n)*?)}'

metadata_files = list(Path(".").rglob("metadata.textproto"))
deviations = []
    
for f in metadata_files:
    with open(f, 'r') as fp:
        data = fp.read()
        matches = re.findall(rx, data)
        for m in matches:
            for d in m[3].split('\n'):
                if d.strip() != "":
                    deviations.append(d.strip())

deviations = list(dict.fromkeys(deviations))
deviations.sort()
print (*deviations, sep="\n")
        
