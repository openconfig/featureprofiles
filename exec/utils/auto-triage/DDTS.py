from pymongo import MongoClient

class DDTS:
    def __init__(self):
        self._username = "qddtsadmin"
        self._password = "$QddtsAdmin"

        self._client = MongoClient(f"mongodb://{self._username}:{self._password}@sjc-p-qddts-d13")       
        self._database = self._client["bugdata"]
        self._ddts = self._database["qddtsdata"]

    def search(self, id):
        query = { "_id": id }
        projection = { "_id": 1, "Status": 1, "Submitted-on": 1, "CLOSED": 1 }

        return self._ddts.find_one(query, projection)
        