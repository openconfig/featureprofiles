from pymongo import MongoClient

class Database:
    def __init__(self):
        self._client = MongoClient("mongodb://xr-sf-npi-lnx.cisco.com:27017/")
        self._database = self._client["auto-triage"]

        self._data = self._database["data"]
        self._labels = self._database["labels"]
        self._firex_ids = self._database["firex-ids"]
        self._groups = self._database["groups"]

    def insert_logs(self, documents):
        if(len(documents) == 0):
            return
        self._data.insert_many(documents)

    def insert_metadata(self, document = {}):
        if(document == {}):
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
                            "testcases.label": {"$in": valid_labels}
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

