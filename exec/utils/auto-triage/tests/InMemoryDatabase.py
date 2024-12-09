import datetime
import logging
import os

import sys
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from IDatabase import IDatabase

logger = logging.getLogger(__name__)

class InMemoryDatabase(IDatabase):
    def __init__(self):
        self.data = []
        self.labels = []
        self.firex_ids = []
        self.groups = []

    def insert_logs(self, documents):
        if not documents:
            logger.debug("Called InMemoryDatabase.insert_logs() and No Documents to Insert into File")
            return    
        self.data.extend(documents)

    def insert_metadata(self, document={}):
        if not document:
            logger.debug("Called InMemoryDatabase.insert_metadata() and No Metadata to Insert into File")
            return
        self.firex_ids.append(document)

    def get_datapoints(self):
        results = []
        for doc in self.data:
            if 'testcases' in doc:
                for testcase in doc['testcases']:
                    if testcase.get('label') in self.labels and testcase.get('status') == 'failed':
                        result = {
                            'name': testcase.get('name'),
                            'plan_id': doc.get('plan_id'),
                            'logs': testcase.get('logs'),
                            'timestamp': doc.get('timestamp'),
                            'label': testcase.get('label'),
                        }
                        results.append(result)

        return results

    def is_subscribed(self, name):
        return name in self.groups

    def get_historical_testsuite(self, lineup, group, plan):
        filtered_documents = [doc for doc in self.data if doc.get('group') == group and doc.get('plan_id') == plan and doc.get('lineup') == lineup]
        sorted_documents = sorted(filtered_documents, key=lambda x: x.get('timestamp', ''), reverse=True)

        if sorted_documents:
            document = sorted_documents[0]
            result = {
                'testcases': document.get('testcases', []),
                'bugs': document.get('bugs', [])
            }
            logger.debug(f"Called InMemoryDatabase.get_historical_testsuite() on {group}/{plan} and received: {result}")
            return result
        
        logger.debug(f"Called InMemoryDatabase.get_historical_testsuite() on {group}/{plan} and received: None")
        return None