import os
from pymongo import MongoClient
from datetime import datetime
from dotenv import load_dotenv  

load_dotenv()

class DDTS:
    """Class to Handle DDTS Bug Types"""
    def __init__(self):
        # Get database credentials and connection info from environment variables
        self._username = os.environ.get("QDDTS_USER")
        self._password = os.environ.get("QDDTS_PASSWORD")
        db_name = os.environ.get("QDDTS_DB")
        collection_name = os.environ.get("QDDTS_COLLECTION")
        
        # Use the full URI if available, otherwise build it from parts
        uri = os.environ.get("QDDTS_URI")
        if not uri:
            uri = f"mongodb://{self._username}:{self._password}@sjc-p-qddts-d13"

        # Access DDTS MongoDB Cluster
        self._client = MongoClient(uri)
        self._database = self._client[db_name]
        self._ddts = self._database[collection_name]

    def _search(self, name):
        """Search for a given DDTS and return desired attributes"""
        # Extract the DDTS ID if it's a full URL
        ddts_id = name
        if "cdetsng.cisco.com" in name:
            # Use regex to extract the CSC ID from the URL
            import re
            match = re.search(r'CSC.*', name)
            if match:
                ddts_id = match.group(0)
            else:
                print(f"Could not extract DDTS ID from URL: {name}")
                return None

        query = { "_id": ddts_id }
        projection = { "_id": 1, "Status": 1, "Submitted-on": 1, 
                    "Submitter": 1, "Severity": 1, "CLOSED": 1 }

        document = self._ddts.find_one(query, projection)
        print(f"Called DDTS._search() on {name} and received: {document}")

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
        Returns None if:
        - The bug status is R, V, U, or C (indicating it should not be tracked)
        
        Sets resolved=true if:
        - The bug has status R or V
        - OR the bug has a CLOSED field
        
        Includes the age of the DDTS bug in days.
        """
        # Get the full DDTS details from MongoDB
        document = self._search(name)
        
        # Skip bugs with status R, V, U, or C
        if document and document.get("Status") in ["R", "V", "U", "C"]:
            return None
        
        # Check if the bug should be marked as resolved
        is_resolved = False
        
        # Mark as resolved if it has a CLOSED field OR status is R or V
        if document:
            if document.get("CLOSED") is not None:
                is_resolved = True
            if document.get("Status") in ["R", "V"]:
                is_resolved = True
        
        # Create the base bug object
        bug = {
            "name": name,
            "type": "DDTS",
            "username": "Cisco InstaTriage",
            "updated": datetime.now(),
            "resolved": is_resolved
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
