import xml.etree.ElementTree as ET
from pymongo import MongoClient
from langchain_community.vectorstores import FAISS
from langchain_huggingface import HuggingFaceEmbeddings
from langchain_core.documents import Document


class VectorStore:
    def __init__(self):
        client = MongoClient("mongodb://xr-sf-npi-lnx.cisco.com:27017/")
        database = client["auto-triage"]
        self.collection = database["data"]
        self.collection_labels = database["labels"]
        self.firex_ids_collection = database["firex-ids"]

        self.tf = HuggingFaceEmbeddings(model_name="all-MiniLM-L6-v2")

        self.vector_store = None

        self._create_index()

    def _create_index(self):
        results = list(
            self.collection.aggregate(
                [
                    {"$unwind": "$testcases"},
                    {
                        "$match": {
                            "testcases.status": {"$eq": "failed"},
                            "testcases.generated": {"$eq": False},
                            "testcases.label": {"$ne": ""},
                            "$and": [
                                {"testcases.logs": {"$ne": ""}},
                                {"testcases.logs": {"$ne": "None"}},
                            ],
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
        visited = set()

        for document in documents:
            data, score = document
            if data.metadata["label"] in visited or score < 0:
                continue
            labels.append({"label": data.metadata["label"], "score": score})
            visited.add(data.metadata["label"])

        return labels

    def create_documents(self, file, group, efr, run_id, lineup):
        documents = list()

        tree = ET.parse(file)

        testsuites_metadata = tree.getroot().attrib

        if int(testsuites_metadata["failures"]) > 0:
            meta_firex = {
                "firex_id": run_id,
                "group": group,
                "lineup": lineup,
                "tag": efr,
            }
            meta_firex.update(testsuites_metadata)
            self.firex_ids_collection.insert_one(meta_firex)

        for testsuite in tree.getroot().findall("./testsuite"):
            stats = testsuite.attrib
            properties = testsuite.find("properties")
            testcases = testsuite.findall("testcase")

            data = dict()

            data["group"] = meta_firex["group"]
            data["efr"] = meta_firex["tag"]
            data["run_id"] = meta_firex["firex_id"]
            data["lineup"] = meta_firex["lineup"]
            data["tests"] = int(stats.get("tests", 0))
            data["failures"] = int(stats.get("failures", 0))
            data["errors"] = int(stats.get("errors", 0))
            data["disabled"] = int(stats.get("disabled", 0))
            data["skipped"] = int(stats.get("skipped", 0))
            data["timestamp"] = str(stats.get("timestamp", 0))
            data["testcases"] = list()

            keys = [
                "test.plan_id",
                "test.description",
                "test.uuid",
                "testsuite_hash",
                "testsuite_root",
            ]

            for property in properties:
                if property.get("name") in keys:
                    data[property.get("name").replace("test.", "")] = property.get(
                        "value"
                    )

            if data["failures"] == 0:
                continue

            for testcase in testcases:

                testcase_data = dict()
                failure = testcase.find("failure")

                if failure is not None:
                    testcase_data["name"] = testcase.get("name")
                    testcase_data["time"] = float(testcase.get("time"))
                    testcase_data["message"] = failure.get("message")
                    testcase_data["logs"] = str(failure.text).strip()
                    testcase_data["status"] = "failed"
                    testcase_data["triage_status"] = "New"

                    labels = self._generate_labels(
                        failure.text if failure.text is not None else "",
                    )

                    if len(labels) > 0:
                        testcase_data["generated_labels"] = self._generate_labels(
                            failure.text if failure.text is not None else "",
                        )
                        testcase_data["generated"] = True
                        testcase_data["label"] = testcase_data["generated_labels"][0]["label"]
                    else:
                        testcase_data["label"] = ""

                else:
                    testcase_data["name"] = testcase.get("name")
                    testcase_data["time"] = float(testcase.get("time"))
                    testcase_data["status"] = "passed"
                    testcase_data["label"] = "Test Passed. No Label Required."

                data["testcases"].append(testcase_data)
            documents.append(data)
        return documents

    def insert_many(self, documents):
        self.collection.insert_many(documents)
