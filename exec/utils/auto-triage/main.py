import argparse

from Database import Database
from FireX import FireX
from Vectorstore import Vectorstore
from DDTS import DDTS


parser = argparse.ArgumentParser(description='Inject FireX Run Results in MongoDB')
parser.add_argument('run_id', help="FireX Run ID")
parser.add_argument('xunit_file', help="XUnit Result File")
args = parser.parse_args()

database = Database()
firex = FireX()
vectorstore = Vectorstore()
ddts = DDTS()

def main():
    # Get Metdata from run.json
    run_info = firex.get_run_information(args.xunit_file)

    # Only Consider Subscribed Groups
    if database.is_subscribed(run_info["group"]) == False:
        return
    
    # Add FireX Metadata
    database.insert_metadata(run_info)
    
    # Create FAISS Index
    datapoints = database.get_datapoints()
    vectorstore.create_index(datapoints)
    
    # Add Testsuite Data
    documents = firex.get_testsuites(vectorstore, database, args.xunit_file, run_info)

    database.insert_logs(documents)

main()
