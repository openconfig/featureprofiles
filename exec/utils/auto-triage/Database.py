from pymongo import MongoClient
from bson import ObjectId
import datetime

from IDatabase import IDatabase

class Database(IDatabase):
    def __init__(self, environment):
        self._client = MongoClient("mongodb://xr-sf-npi-lnx.cisco.com:27017/")
        self._database = self._client[environment]

        self._data = self._database["data"]
        self._labels = self._database["labels"]
        self._firex_ids = self._database["firex-ids"]
        self._groups = self._database["groups"]

    def insert_logs(self, documents):
        if(len(documents) == 0):
            print("Called Database.insert_logs() and No Documents to Insert into MongoDB")
            return
        self._data.insert_many(documents)

    def insert_metadata(self, document = {}):
        if(document == {}):
            print("Called Database.insert_metadata() and No Metadata to Insert into MongoDB")
            return
        self._firex_ids.insert_one(document)

    def get_datapoints(self):
        documents = list(self._labels.find(filter = {}, projection = {
            "_id": 0,
            "label": 1
        }))

        valid_labels = [document["label"] for document in documents]

        results = list(
            self._data.aggregate(
                [
                    {"$unwind": "$testcases"},
                    {
                        "$match": {
                            "testcases.label": {"$in": valid_labels},
                            "testcases.status": "failed",
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
        document = self._groups.find_one(filter = {
            "group": name
        })

        if document:
            return True
        return False

    def get_historical_testsuite(self, lineup, group, plan):
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
