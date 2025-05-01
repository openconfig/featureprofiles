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
            logger.debug("Called InMemoryDatabase.insert_logs() and No Documents to Insert")
            return
        self.data.extend(documents)

    def insert_metadata(self, document={}):
        if not document:
            logger.debug("Called InMemoryDatabase.insert_metadata() and No Metadata to Insert")
            return
        self.firex_ids.append(document)

    def get_datapoints(self):
        results = []
        for doc in self.data:
            if 'testcases' in doc:
                for testcase in doc.get('testcases', []):
                    # Check label exists and is valid before checking status
                    label = testcase.get('label')
                    if label and label in self.labels and testcase.get('status') == 'failed':
                        result = {
                            'name': testcase.get('name'),
                            'plan_id': doc.get('plan_id'),
                            'logs': testcase.get('logs'),
                            'timestamp': doc.get('timestamp'),
                            'label': label,
                        }
                        results.append(result)
        return results

    def is_subscribed(self, name):
        return name in self.groups

    # Modified get_historical_testsuite to accept before_timestamp
    def get_historical_testsuite(self, lineup, group, plan, before_timestamp=None):
        """Find historical testsuite based on given parameters, optionally before a specific timestamp."""
        filtered_documents = [
            doc for doc in self.data if (
                doc.get('group') == group and
                doc.get('plan_id') == plan and
                doc.get('lineup') == lineup and
                # Add timestamp filtering logic
                (before_timestamp is None or doc.get('timestamp', '') < before_timestamp)
            )
        ]
        # Sort descending by timestamp
        # Handle potential None timestamps during sorting
        sorted_documents = sorted(
            filtered_documents,
            key=lambda x: x.get('timestamp', ''), # Use empty string for None for comparison
            reverse=True
        )

        if sorted_documents:
            document = sorted_documents[0]
            result = {
                'testcases': document.get('testcases', []),
                'bugs': document.get('bugs', []),
                'timestamp': document.get('timestamp'),
                'run_id': document.get('run_id')
            }
            logger.debug(f"InMemoryDB.get_historical_testsuite({group}/{plan}/{lineup}, before={before_timestamp}) -> run_id: {result.get('run_id', 'N/A')}")
            return result

        logger.debug(f"InMemoryDB.get_historical_testsuite({group}/{plan}/{lineup}, before={before_timestamp}) -> None")
        return None