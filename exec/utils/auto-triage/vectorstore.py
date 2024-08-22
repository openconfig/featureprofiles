import xml.etree.ElementTree as ET
from pymongo import MongoClient
from langchain_community.vectorstores import FAISS
from langchain_huggingface import HuggingFaceEmbeddings
from langchain_core.documents import Document


class VectorStore:
    def __init__(self):
        client = MongoClient("mongodb://xr-sf-npi-lnx.cisco.com:27017/")
        database = client["auto-triage"]
        self.collection = database["sample-data"]
        self.collection_labels = database["sample-labels"]

        self.tf = HuggingFaceEmbeddings(model_name="all-MiniLM-L6-v2")

        self._create_index()

    def _create_index(self):
        results = list(
            self.collection.aggregate(
                [
                    {
                        '$match': {
                            'label': {
                                '$ne': ''
                            }, 
                            'status': {
                                '$eq': 'failed'
                            }, 
                            '$and': [
                                {
                                    'logs': {
                                        '$ne': ''
                                    }
                                }, {
                                    'logs': {
                                        '$ne': 'None'
                                    }
                                }
                            ]
                        }
                    }, {
                        '$project': {
                            'name': True, 
                            'plan_id': True, 
                            'logs': True, 
                            'timestamp': True, 
                            'label': True
                        }
                    }
                ]
            )
        )

        documents = [
            Document(
                page_content=x["logs"],
                metadata={
                    "name": x["name"],
                    "plan_id": x["plan_id"],
                    "timestamp": x["timestamp"],
                    "label": x["label"],
                },
            )
            for x in results
        ]

        if len(documents) > 0:
            self.vector_store = FAISS.from_documents(documents, self.tf)
        else: 
            self.vector_store = None

    def _generate_labels(self, sentence):

        if self.vector_store == None or sentence == None:
            return []

        documents = self.vector_store.similarity_search_with_relevance_scores(
            sentence, k=4
        )

        labels = list()

        for document in documents:
            data, score = document
            labels.append({"label": data.metadata["label"], "score": score})

        return labels

    def create_documents(self, file, group, efr, run_id, image):
        documents = list()

        tree = ET.parse(file)

        for testsuite in tree.getroot().findall('./testsuite'):
            stats = testsuite.attrib
            properties = testsuite.find("properties")
            testcases = testsuite.findall("testcase")

            for testcase in testcases:
                
                data = dict()

                data["group"] = group
                data["efr"] = efr
                data["run_id"] = run_id
                data["image"] = image
                data["tests"] = int(stats.get("tests", 0))
                data["failures"] = int(stats.get("failures", 0))
                data["errors"] = int(stats.get("errors", 0))
                data["disabled"] = int(stats.get("disabled", 0))
                data["skipped"] = int(stats.get("skipped", 0))
                data["timestamp"] = str(stats.get("timestamp", 0))

                keys = ["test.plan_id", "test.description", "test.uuid"]

                for property in properties:
                    if property.get("name") in keys:
                        data[property.get("name").replace("test.", "")] = property.get(
                            "value"
                        )

                if data["failures"] == 0:
                    continue

                failure = testcase.find("failure")

                if failure is not None:
                    data["name"] = testcase.get("name")
                    data["time"] = float(testcase.get("time"))
                    data["message"] = failure.get("message")
                    data["logs"] = str(failure.text).strip()
                    data["status"] = "failed"

                    labels = self._generate_labels(
                        failure.text if failure.text is not None else "",
                    )

                    if len(labels) > 0:
                        data["generated_labels"] = self._generate_labels(
                            failure.text if failure.text is not None else "",
                        )
                        data["generated"] = True
                        data["label"] = data["generated_labels"][0]["label"]
                    else:
                        data["label"] = ""

                else:
                    data["name"] = testcase.get("name")
                    data["time"] = float(testcase.get("time"))
                    data["status"] = "passed"
                    data["label"] = "Test Passed. No Label Required."

                documents.append(data)
        return documents

    def insert_many(self, documents):
        self.collection.insert_many(documents)
