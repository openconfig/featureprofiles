from FireX import FireX
from Vectorstore import Vectorstore

class CIT:
    def __init__(self, db):
        self.db = db

    def process_run(self, xunit_root, version='', workspace=''):
        firex = FireX(xunit_root)
        
        vectorstore = Vectorstore()
    
        group = firex.get_group()

        # Get Metdata from run.json
        run_info = firex.get_run_information(version, workspace)

        # Only Consider Subscribed Groups
        if not self.db.is_subscribed(group):
            raise Exception(f"{run_info['group']} is not a subscribed group. Please subscribe via the CIT Dashboard")

        self.db.insert_metadata(run_info)
        print("Successfully Inserted Metadata into MongoDB")
        
        # Create FAISS Index
        datapoints = self.db.get_datapoints()
        vectorstore.create_index(datapoints)
        print("Successfully Created FAISS Index")
        
        # Add Testsuite Data
        documents = firex.get_testsuites(vectorstore, self.db, run_info)
        print("Successfully Created Documents")

        self.db.insert_logs(documents)
        print("Successfully Inserted Documents into MongoDB")