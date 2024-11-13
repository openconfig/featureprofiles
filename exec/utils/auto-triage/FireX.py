import xml.etree.ElementTree as ET
import json
import glob

from DDTS import DDTS
from TechZone import TechZone

ddts = DDTS()
techzone = TechZone()

class FireX:
    def get_run_information(self, file, version, workspace):
        tree = ET.parse(file)
        root = tree.getroot()

        testsuite_root = root.find(".//properties/property[@name='testsuite_root']").get("value")
        run_file = testsuite_root + "/run.json"

        if version == "" and workspace == "":
            show_version = glob.glob(testsuite_root + "/tests_logs/*/debug_files/dut/show_version")[0]
            with open(show_version) as show_version_contents:
                lines = show_version_contents.readlines()
                header = lines[0]

                version = header.split(",")[1].split(" ")[2].strip()

                for line in lines:
                    if line.strip().startswith("Workspace"):
                        workspace = line.split(":")[1].strip()

        testsuites_metadata = root.attrib 

        testbed = "Hardware"

        sim_files = glob.glob(testsuite_root + "/testbed_logs/*/bringup_success/sim-config.yaml")
        if(len(sim_files) > 0):
            testbed = "Simulation"

        with open(run_file) as metadata:
            meta = json.load(metadata)

            chain_index = meta["submission_cmd"].index("--chain")

            testsuites_metadata.update({
                "firex_id": meta["firex_id"],
                "group": meta["group"],
                "lineup": meta["inputs"]["lineup"],
                "testbed": testbed,
                "chain": meta["submission_cmd"][chain_index + 1],
                "workspace": workspace,
                "tag": version,
            })
        return testsuites_metadata

    def _create_testsuites(self, vectorstore, testcases, errors_count, historial_testsuite, inherit = False):
        testsuites = []

        for testcase_index in range(len(testcases)):
            testcase = testcases[testcase_index]

            if inherit:
                history = historial_testsuite["testcases"][testcase_index]

            failure = testcase.find("failure")

            testcase_data = {
                "name": testcase.get("name"),
                "time": float(testcase.get("time", 0))
            }

            if errors_count > 0:
                # Aborted
                testcase_data["status"] = "aborted"

                if inherit and history.get("status") == "aborted":
                    testcase_data["triage_status"] = history.get("triage_status", "Resolved")
                    testcase_data["label"] = history["label"]
                else:
                    testcase_data["triage_status"] = "New"
                    testcase_data["label"] = ""
            elif failure is None:
                if testcase.find("skipped"):
                    # Skipped Testcase
                    testcase_data["status"] = "skipped"
                    testcase_data["label"] = "Test Skipped. No Label Required."
                else:    
                    # Passed Testcase
                    testcase_data["status"] = "passed"
                    testcase_data["label"] = "Test Passed. No Label Required."
            else:
                # Failed Testcase
                testcase_data["message"] = failure.get("message")
                testcase_data["logs"] = str(failure.text).strip()

                if inherit and history.get("status") == "failed":
                    testcase_data["triage_status"] = history.get("triage_status", "Resolved")
                    testcase_data["label"] = history["label"]
                else:
                    testcase_data["status"] = "failed"
                    testcase_data["triage_status"] = "New"

                    labels = vectorstore.query(
                        failure.text if failure.text is not None else "",
                    )

                    print(f"Called FireX._create_testsuites() and generated for {testcase.get('name')} the following labels: {labels}")

                    if len(labels) > 0:
                        testcase_data["generated_labels"] = labels
                        testcase_data["generated"] = True
                        testcase_data["label"] = testcase_data["generated_labels"][0]["label"]
                    else:
                        testcase_data["label"] = ""
            testsuites.append(testcase_data)
        return testsuites

    def get_testsuites(self, vectorstore, database, file, run_info):
        tree = ET.parse(file)
        root = tree.getroot()

        documents = []

        for testsuite in root.findall("./testsuite"):         
            stats = testsuite.attrib
            properties = testsuite.find("properties")
            testcases = testsuite.findall("testcase")

            failures_count = int(stats.get("failures", 0))
            errors_count = int(stats.get("errors", 0))

            data = {
                "group": run_info["group"],
                "efr": run_info["tag"],
                "run_id": run_info["firex_id"],
                "lineup": run_info["lineup"],
                "tests": int(stats.get("tests", 0)),
                "failures": failures_count,
                "errors": errors_count,
                "disabled": int(stats.get("disabled", 0)),
                "skipped": int(stats.get("skipped", 0)),
                "timestamp" : str(stats.get("timestamp", 0)),
                "testcases": [],
                "bugs": []
            }

            keys = [
                    "test.plan_id",
                    "test.description",
                    "test.uuid",
                    "testsuite_hash",
                    "testsuite_root",
                ]
            
            cafy_keys_mappings = {
                "testsuite_name": "plan_id",
                "testsuite_hash": "testsuite_hash",
                "testsuite_root": "testsuite_root"
            }

            framework = root.find(".//properties/property[@name='framework']").get("value")

            if framework == "cafy2":
                for property in properties:
                    if property.get("name") in cafy_keys_mappings:
                        data[cafy_keys_mappings[property.get("name")]] = property.get("value")
            else:
                for property in properties:
                    if property.get("name") in keys:
                        data[property.get("name").replace("test.", "")] = property.get(
                            "value"
                        )

            if data.get("plan_id") and data.get("group"):
                existing_bugs, existing = database.inherit_bugs(data['group'], data['plan_id'])

            if existing:
                for bug in existing_bugs[0]["bugs"]:
                    name = bug["name"]
                    if bug["type"] == "DDTS":
                        if ddts.is_open(name):
                            data["bugs"].append(ddts.inherit(name))
                    elif bug["type"] == "TechZone":
                        data["bugs"].append(techzone.inherit(name))
                historial_testsuite = database.get_historical_testsuite(data["group"], data["plan_id"])
                data["testcases"] = self._create_testsuites(vectorstore, testcases, data["errors"], historial_testsuite, inherit = True)
            else:
                data["testcases"] = self._create_testsuites(vectorstore, testcases, data["errors"], None, inherit = False)
            documents.append(data)
        return documents

