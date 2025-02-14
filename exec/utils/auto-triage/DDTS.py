from pymongo import MongoClient
from datetime import datetime

class DDTS:
    """Class to Handle DDTS Bug Types"""
    def __init__(self):
        self._username = "qddtsadmin"
        self._password = "$QddtsAdmin"

        # Access DDTS MongoDB Cluster
        self._client = MongoClient(f"mongodb://{self._username}:{self._password}@sjc-p-qddts-d13")       
        self._database = self._client["bugdata"]
        self._ddts = self._database["qddtsdata"]

    def _search(self, id):
        """Search for a given DDTS and return desired attributes"""
        query = { "_id": id }
        projection = { "_id": 1, "Status": 1, "Submitted-on": 1, "CLOSED": 1 }

        document = self._ddts.find_one(query, projection)
        print(f"Called DDTS._search() on {id} and recieved: {document}")

        return document

    def is_open(self, id):
        """Determine if given DDTS is open. Later to be used for dynamic inheritance."""
        document = self._search(id)
        if document and document.get("CLOSED", None) is None:
            return True
        return False
        
    def inherit(self, name):
        """Create a DDTS bug to inherit"""
        return {
            "name": name,
            "type": "DDTS",
            "username": "Cisco InstaTriage",
            "updated": datetime.now(),
            "resolved": not self.is_open(name)
        }
