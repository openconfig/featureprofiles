import xml.etree.ElementTree as ET
import json


class FireX:
    def get_run_information(self, file):
        tree = ET.parse(file)
        root = tree.getroot()

        testsuite_root = root.find(".//properties/property[@name='testsuite_root']").get("value")
        run_file = testsuite_root + "/run.json"

        testsuites_metadata = root.attrib 

        with open(run_file) as metadata:
            meta = json.load(metadata)

            testsuites_metadata.update({
                "firex_id": meta["firex_id"],
                "group": meta["group"],
                "lineup": meta["inputs"]["lineup"],
                "tag": (
                    meta["inputs"]["tag"]
                    if "tag" in meta["inputs"]
                    else "Unknown"
                ),
            })

        return testsuites_metadata
    
    def get_testsuites(self, vectorstore, file, run_info):
        tree = ET.parse(file)
        root = tree.getroot()

        documents = []

        for testsuite in root.findall("./testsuite"):         
            stats = testsuite.attrib
            properties = testsuite.find("properties")
            testcases = testsuite.findall("testcase")

            failures_count = int(stats.get("failures", 0))

            if failures_count == 0: 
                continue

            data = {
                "group": run_info["group"],
                "efr": run_info["tag"],
                "run_id": run_info["firex_id"],
                "lineup": run_info["lineup"],
                "tests": int(stats.get("tests", 0)),
                "failures": failures_count,
                "errors": int(stats.get("errors", 0)),
                "disabled": int(stats.get("disabled", 0)),
                "skipped": int(stats.get("skipped", 0)),
                "timestamp" : str(stats.get("timestamp", 0)),
                "testcases": []
            }

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

            for testcase in testcases:

                failure = testcase.find("failure")

                testcase_data = {
                    "name": testcase.get("name"),
                    "time": float(testcase.get("time"))
                }

                if failure is None:
                    # Passed Testcase
                    testcase_data["status"] = "passed"
                    testcase_data["label"] = "Test Passed. No Label Required."
                else:
                    # Failed Testcase
                    testcase_data["message"] = failure.get("message")
                    testcase_data["logs"] = str(failure.text).strip()
                    testcase_data["status"] = "failed"
                    testcase_data["triage_status"] = "New"

                    labels = vectorstore.query(
                        failure.text if failure.text is not None else "",
                    )

                    if len(labels) > 0:
                        testcase_data["generated_labels"] = labels
                        testcase_data["generated"] = True
                        testcase_data["label"] = testcase_data["generated_labels"][0]["label"]
                    else:
                        testcase_data["label"] = ""

                data["testcases"].append(testcase_data)
            documents.append(data)
        return documents

