import json
import glob
import os

from DDTS import DDTS
from TechZone import TechZone
from Github import Github

ddts = DDTS()
techzone = TechZone()
github = Github()

class FireX:
    def __init__(self, xml_root):        
        self.root = xml_root 
        self.testsuite_root = self.root.find(".//properties/property[@name='testsuite_root']").get("value")
        run_info_file = os.path.join(self.testsuite_root, "run.json")
        with open(run_info_file) as f:
            self.run_info = json.load(f)

    def get_group(self):
        return self.run_info["group"]

    def get_run_information(self, version, workspace):
        if version == "" and workspace == "":
            #TODO: need a better generic way
            show_version = glob.glob(self.testsuite_root + "/tests_logs/*/debug_files/dut*/show_version")[0]
            with open(show_version) as show_version_contents:
                lines = show_version_contents.readlines()
                header = lines[0]

                version = header.split(",")[1].split(" ")[2].strip()

                for line in lines:
                    if line.strip().startswith("Workspace"):
                        workspace = line.split(":")[1].strip()

        testsuites_metadata = self.root.attrib 

        testbed = "Hardware"
        sim_files = glob.glob(self.testsuite_root + "/testbed_logs/*/bringup_success/sim-config.yaml")
        if(len(sim_files) > 0):
            testbed = "Simulation"

        chain_index = self.run_info["submission_cmd"].index("--chain")

        testsuites_metadata.update({
            "firex_id": self.run_info["firex_id"],
            "group": self.run_info["group"],
            "lineup": self.run_info["inputs"]["lineup"],
            "testbed": testbed,
            "chain": self.run_info["submission_cmd"][chain_index + 1],
            "workspace": workspace,
            "tag": version,
        })

        return testsuites_metadata

    def _create_testsuites(self, vectorstore, testcases, historial_testsuite):
        testsuites = []
    
        for testcase in testcases:
            inherit = historial_testsuite != None   
            if inherit:
                history = None
                for e in historial_testsuite["testcases"]:
                    if e.get("name") == testcase.get("name"):
                        history = e
                        break
                inherit = history != None

            testcase_data = {
                "name": testcase.get("name"),
                "time": float(testcase.get("time", 0))
            }

            failure_el = testcase.find("failure")
            error_el = testcase.find("error")
            skipped_el = testcase.find("skipped")

            # Skipped Testcase
            if skipped_el != None:
                testcase_data["status"] = "skipped"
                testcase_data["label"] = "Test Skipped. No Label Required."
            # Aborted
            elif error_el != None and error_el.get("message") is None:
                if inherit and history.get("status") == "aborted":
                    testcase_data["triage_status"] = history.get("triage_status", "New")
                    testcase_data["label"] = history.get("label", "")
                    testcase_data["bugs"] = history.get("bugs", [])
                else:
                    testcase_data["triage_status"] = "New"
                    testcase_data["label"] = ""
            # Failed Testcase
            elif (error_el != None and error_el.get("message")) or failure_el != None:
                text = error_el.text if error_el != None else failure_el.text

                testcase_data["message"] = "Failed"
                testcase_data["logs"] = str(text).strip()

                if inherit and history.get("status") == "failed":
                    testcase_data["triage_status"] = history.get("triage_status", "New")
                    testcase_data["label"] = history.get("label", "")
                    testcase_data["status"] = "failed"
                    testcase_data["bugs"] = history.get("bugs", [])
                else:
                    testcase_data["status"] = "failed"
                    testcase_data["triage_status"] = "New"

                    labels = vectorstore.query(
                        text if text is not None else "",
                    )

                    print(f"Called FireX._create_testsuites() and generated for {testcase.get('name')} the following labels: {labels}")

                    if len(labels) > 0:
                        testcase_data["generated_labels"] = labels
                        testcase_data["label"] = testcase_data["generated_labels"][0]["label"]
                        if testcase_data["generated_labels"][0]["score"] > 0.9:
                            testcase_data["generated"] = False
                            testcase_data["triage_status"] = "Resolved"
                        else:
                            testcase_data["generated"] = True
                    else:
                        testcase_data["label"] = ""
            # Passed Testcase
            else:
                testcase_data["status"] = "passed"
                testcase_data["label"] = "Test Passed. No Label Required."
                
            testsuites.append(testcase_data)
        return testsuites

    def get_testsuites(self, vectorstore, database, run_info):
        documents = []

        for testsuite in self.root.findall("./testsuite"):         
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

            framework = self.root.find(".//properties/property[@name='framework']").get("value")

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

            historial_testsuite = None
            if set(["plan_id", "group", "lineup"]).issubset(data.keys()):
                historial_testsuite = database.get_historical_testsuite(data['lineup'], data["group"], data["plan_id"])
        
            if historial_testsuite:
                for bug in historial_testsuite.get("bugs", []):
                    name = bug["name"]
                    if bug["type"] == "DDTS":
                        data["bugs"].append(ddts.inherit(name))
                    elif bug["type"] == "TechZone":
                        data["bugs"].append(techzone.inherit(name))
                    elif bug["type"] == "Github":
                        data["bugs"].append(github.inherit(name))

            data["testcases"] = self._create_testsuites(vectorstore, testcases, historial_testsuite)
            documents.append(data)
        return documents

