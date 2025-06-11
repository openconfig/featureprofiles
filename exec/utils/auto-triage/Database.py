# auto-triage/Database.py
from pymongo import MongoClient, errors
import logging
from IDatabase import IDatabase
from datetime import datetime, timedelta

logger = logging.getLogger(__name__)

class Database(IDatabase):
    """MongoDB Handler"""
    def __init__(self, environment, no_upload=False):
        """Connect to MongoDB Community Cisco Cluster"""
        self._environment = environment
        self._no_upload = no_upload
        self._client = None
        self._database = None
        self._data = None
        self._labels = None
        self._firex_ids = None
        self._groups = None

        try:
            self._client = MongoClient("mongodb://xr-sf-npi-lnx:27017,oc-perf-lnx:27017,sjc-ads-9292.cisco.com:27017/?replicaSet=xr-sf-npi-replication-to-oc-perf-lnx", serverSelectionTimeoutMS=5000)
            self._client.admin.command('ismaster')
            logger.info(f"Successfully connected to MongoDB server for environment: {self._environment}")
            self._database = self._client[self._environment]
            self._data = self._database["data"]
            self._labels = self._database["labels"]
            self._firex_ids = self._database["firex-ids"]
            self._groups = self._database["groups"]
        except errors.ConnectionFailure as e:
            logger.error(f"Could not connect to MongoDB for environment {self._environment}: {e}")
            if not self._no_upload:
                 raise ConnectionError(f"MongoDB connection failed for {self._environment} and uploads are required.") from e
            else:
                 logger.warning("MongoDB connection failed, but --no-upload is active. Reads will likely fail.")

    def insert_logs(self, documents):
        """Insert documents (testsuites) into MongoDB collection: data"""
        if not documents:
            logger.warning("Called Database.insert_logs() and No Documents to Insert into MongoDB")
            return
        if self._no_upload:
            logger.info("[NO UPLOAD MODE] Skipping insert_logs for %d documents.", len(documents))
            return
        if self._data is None:
             logger.error("MongoDB collection 'data' not initialized (connection likely failed). Cannot insert logs.")
             return
        try:
            logger.info(f"Inserting {len(documents)} documents into MongoDB collection 'data'...")
            self._data.insert_many(documents)
            logger.info("Successfully inserted documents.")
        except Exception as e:
            logger.error(f"Failed to insert documents into 'data': {e}")

    def insert_metadata(self, document={}):
        """Insert FireX Metadata into MongoDB collection: firex_ids"""
        if not document:
            logger.warning("Called Database.insert_metadata() and No Metadata to Insert into MongoDB")
            return
        if self._no_upload:
            logger.info("[NO UPLOAD MODE] Skipping insert_metadata for document: %s", document.get('firex_id', 'N/A'))
            return
        if self._firex_ids is None:
             logger.error("MongoDB collection 'firex-ids' not initialized (connection likely failed). Cannot insert metadata.")
             return
        try:
            logger.info(f"Inserting metadata for run {document.get('firex_id', 'N/A')} into MongoDB collection 'firex-ids'...")
            self._firex_ids.insert_one(document)
            logger.info("Successfully inserted metadata.")
        except Exception as e:
            logger.error(f"Failed to insert metadata: {e}")

    def get_datapoints(self):
        """Retrieve valid datapoints to create FAISS index"""
        if self._labels is None or self._data is None:
             logger.error("MongoDB collection 'labels' or 'data' not initialized. Cannot get datapoints.")
             return []
        try:
            documents = list(self._labels.find(filter={}, projection={"_id": 0, "label": 1}))
            valid_labels = [doc["label"] for doc in documents if "label" in doc]
            results = list(
                self._data.aggregate([
                    {"$unwind": "$testcases"},
                    {"$match": {
                        "testcases.label": {"$in": valid_labels},
                        "testcases.status": "failed",
                        "timestamp": {"$gte": datetime.now() - timedelta(days=30)}
                    }},
                    {"$project": {
                        "_id": 0, "name": "$testcases.name", "plan_id": "$plan_id",
                        "logs": "$testcases.logs", "timestamp": "$timestamp",
                        "label": "$testcases.label",
                    }}
                ])
            )
            return results
        except Exception as e:
             logger.error(f"Failed to execute aggregation pipeline for get_datapoints: {e}")
             return []

    def is_subscribed(self, name):
        """Determine if the given group name is subscribed to CIT"""
        if self._groups is None:
             logger.error("MongoDB collection 'groups' not initialized. Cannot check subscription.")
             return False
        try:
            document = self._groups.find_one(filter={"group": name})
            return document is not None
        except Exception as e:
            logger.error(f"Failed query 'groups' for subscription check for group '{name}': {e}")
            return False

    # Modified get_historical_testsuite to accept before_timestamp
    def get_historical_testsuite(self, lineup, group, plan, before_timestamp=None):
        """Find historical testsuite based on given parameters, optionally before a specific timestamp."""
        if self._data is None:
             logger.error("MongoDB collection 'data' not initialized. Cannot get historical testsuite.")
             return None

        filter_criteria = {
            "group": group,
            "plan_id": plan,
            "lineup": lineup
        }

        if before_timestamp:
            filter_criteria["timestamp"] = {"$lt": before_timestamp}


        projection = {
            "_id": 0, "testcases": 1, "bugs": 1,
            "timestamp": 1, "run_id": 1  
        }
        sort_order = [["timestamp", -1]] 

        try:
            # find_one gets the most recent document matching the (potentially time-filtered) criteria
            document = self._data.find_one(filter=filter_criteria, projection=projection, sort=sort_order)
            logger.debug(f"get_historical_testsuite({group}/{plan}/{lineup}, before={before_timestamp}) -> run_id: {document.get('run_id', 'N/A') if document else 'None'}")
            return document
        except Exception as e:
            # Log the specific error and parameters for easier debugging
            logger.error(f"Failed query in get_historical_testsuite for {group}/{plan}/{lineup} (before={before_timestamp}): {e}")
            return None
