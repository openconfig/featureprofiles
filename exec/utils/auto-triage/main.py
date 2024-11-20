import argparse

from Database import Database
from FireX import FireX
from Vectorstore import Vectorstore
from DDTS import DDTS

parser = argparse.ArgumentParser(description='Inject FireX Run Results in MongoDB')
parser.add_argument('run_id', help="FireX Run ID")
parser.add_argument('xunit_file', help="XUnit Result File")
parser.add_argument('--version',  default='', help="OS Version")
parser.add_argument('--workspace',  default='', help="Workspace")
parser.add_argument('--dev',  default="false", help="Development")
args = parser.parse_args()

production = Database("auto-triage")
development = Database("auto-triage-dev")

firex = FireX()
vectorstore = Vectorstore()
ddts = DDTS()

def main():
    # Get Metdata from run.json
    run_info = firex.get_run_information(args.xunit_file, args.version, args.workspace)

    # Only Consider Subscribed Groups
    if production.is_subscribed(run_info["group"]) == False:
        print(f"{run_info['group']} is not a subscribed group. Please subscribe via the CIT Dashboard")
        return
    
    # Add FireX Metadata
    if args.dev == "false":
        production.insert_metadata(run_info)
    development.insert_metadata(run_info)
    print("Successfully Inserted Metadata into MongoDB")
    
    # Create FAISS Index
    datapoints = production.get_datapoints()
    vectorstore.create_index(datapoints)
    print("Successfully Created FAISS Index")
    
    # Add Testsuite Data
    documents = firex.get_testsuites(vectorstore, production, args.xunit_file, run_info)
    print("Successfully Created Documents")

    if args.dev == "false":
        production.insert_logs(documents)
    development.insert_logs(documents)
    print("Successfully Inserted Documents into MongoDB")

main()
