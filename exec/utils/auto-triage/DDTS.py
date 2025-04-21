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
        projection = { "_id": 1, "Status": 1, "Submitted-on": 1, 
                      "Submitter": 1, "Severity": 1, "CLOSED": 1 }

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
        """
        Create a DDTS bug to inherit with additional information.
        Returns None if the bug status is R, V, U, or C (indicating it should not be tracked).
        Includes the age of the DDTS bug in days.
        """
        # Get the full DDTS details from MongoDB
        document = self._search(name)
        
        # Skip bugs with status R, V, U, or C
        if document and document.get("Status") in ["R", "V", "U", "C"]:
            return None
        
        # Create the base bug object
        bug = {
            "name": name,
            "type": "DDTS",
            "username": "Cisco InstaTriage",
            "updated": datetime.now(),
            "resolved": not self.is_open(name)
        }
        
        # Add additional fields if document exists and contains the information
        if document:
            if "Status" in document:
                bug["Status"] = document["Status"]
            if "Submitter" in document:
                bug["Submitter"] = document["Submitter"]
            if "Severity" in document:
                bug["Severity"] = document["Severity"]
            
            # Calculate DDTS age in days if submission date is available
            if "Submitted-on" in document and document["Submitted-on"]:
                try:
                    submission_date = document["Submitted-on"]
                    if isinstance(submission_date, datetime):
                        # Calculate age in days
                        age_days = (datetime.now() - submission_date).days
                        bug["ddts_age"] = age_days
                except Exception as e:
                    print(f"Error calculating DDTS age for {name}: {e}")
        
        return bug
