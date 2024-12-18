import xml.etree.ElementTree as ET
import argparse

from Database import Database
from CIT import CIT

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Inject FireX Run Results in MongoDB')
    parser.add_argument('run_id', help="FireX Run ID")
    parser.add_argument('xunit_file', help="XUnit Result File")
    parser.add_argument('--version',  default='', help="OS Version")
    parser.add_argument('--workspace',  default='', help="Workspace")
    parser.add_argument('--dev',  default=False, help="Development")
    args = parser.parse_args()

    if args.dev:
        db = Database("auto-triage-dev")
    else: 
        db = Database("auto-triage")

    tree = ET.parse(args.xunit_file)
    CIT(db).process_run(tree.getroot(), args.version, args.workspace)