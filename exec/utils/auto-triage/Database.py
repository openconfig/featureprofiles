from pymongo import MongoClient

from IDatabase import IDatabase
from datetime import datetime, timedelta

class Database(IDatabase):
    """MongoDB Handler"""
    def __init__(self, environment):
        """Connect to MongoDB Community Cisco Cluster"""
        self._client = MongoClient("mongodb://xr-sf-npi-lnx.cisco.com:27017/")
        self._database = self._client[environment]

        # Track desired collections
        self._data = self._database["data"]
        self._labels = self._database["labels"]
        self._firex_ids = self._database["firex-ids"]
        self._groups = self._database["groups"]

    def insert_logs(self, documents):
        """Insert documents (testsuites) into MongoDB collection: data"""
        if(len(documents) == 0):
            print("Called Database.insert_logs() and No Documents to Insert into MongoDB")
            return
        self._data.insert_many(documents)

    def insert_metadata(self, document = {}):
        """Insert FireX Metadata into MongoDB collection: firex_ids"""
        if(document == {}):
            print("Called Database.insert_metadata() and No Metadata to Insert into MongoDB")
            return
        self._firex_ids.insert_one(document)

    def get_datapoints(self):
        """Retrieve valid datapoints to create FAISS index"""
        documents = list(self._labels.find(filter = {}, projection = {
            "_id": 0,
            "label": 1
        }))

        valid_labels = [document["label"] for document in documents]

        # Aggregation pipeline to grab labeled failed testcases
        results = list(
            self._data.aggregate(
                [
                    {"$unwind": "$testcases"},
                    {
                        "$match": {
                            "testcases.label": {"$in": valid_labels},
                            "testcases.status": "failed",
                            #FIXME: tmp fix until we move this into a service
                            "timestamp": {"$gte": datetime.now() - timedelta(days=30)}
                        }
                    },
                    {
                        "$project": {
                            "name": "$testcases.name",
                            "plan_id": 1,
                            "logs": "$testcases.logs",
                            "timestamp": 1,
                            "label": "$testcases.label",
                        }
                    },
                ]
            )
        )

        return results

    def is_subscribed(self, name):
        """Determine if the given group name is subscribed to CIT"""
        document = self._groups.find_one(filter = {
            "group": name
        })

        if document:
            return True
        return False

    def get_historical_testsuite(self, lineup, group, plan):
        """Find historical testsuite based on given parameters to use for dynamic inheritance"""
        filter = {
            "group": group,
            "plan_id": plan,
            "lineup": lineup
        }

        projection = {
            "testcases": 1,
            "bugs": 1
        }
        
        sort = [["timestamp", -1]]

        document = self._data.find_one(filter = filter, projection = projection, sort = sort)
        print(f"Called Database.get_historical_testsuite() on {group}/{plan} and recieved: {document}")

        return document
