# auto-triage/FireX.py
import json
import glob
import os
import logging
import numpy as np

from DDTS import DDTS
from TechZone import TechZone
from Github import Github
from Embeddings import Embeddings # Import Embeddings class

# Import similarity calculation function
from sklearn.metrics.pairwise import cosine_similarity

# Get a logger instance
logger = logging.getLogger(__name__)

ddts = DDTS()
techzone = TechZone()
github = Github()

# Define a constant for max backtracking depth
MAX_VERIFICATION_LOOKUP_DEPTH = 10 

class FireX:
   
    def __init__(self, xml_root):
        """Track FireX files and store run info from run.json"""
        self.root = xml_root
        testsuite_root_property = self.root.find(".//properties/property[@name='testsuite_root']")
        if testsuite_root_property is None or testsuite_root_property.get("value") is None: raise ValueError("Could not find 'testsuite_root' property in xunit data.")
        self.testsuite_root = testsuite_root_property.get("value")
        run_info_file = os.path.join(self.testsuite_root, "run.json")
        if not os.path.exists(run_info_file): raise FileNotFoundError(f"run.json not found at {run_info_file}")
        try:
            with open(run_info_file) as f: self.run_info = json.load(f)
        except json.JSONDecodeError as e: raise ValueError(f"Error decoding JSON from {run_info_file}: {e}") from e
        try:
            logger.info("Initializing embedding model...")
            self.embeddings = Embeddings(); self.embedding_model = self.embeddings.get_model()
            logger.info("Embedding model initialized successfully.")
        except Exception as e:
            logger.error(f"Failed to initialize embedding model: {e}", exc_info=True); self.embedding_model = None

    def get_group(self):
        return self.run_info.get("group", "Unknown")

    def get_run_information(self, version, workspace):
        if version == "" and workspace == "":
            show_version_files = glob.glob(self.testsuite_root + "/tests_logs/*/debug_files/dut*/show_version")
            if show_version_files:
                show_version = show_version_files[0]; version, workspace = self._parse_show_version(show_version)
            else: logger.warning("show_version file not found...")
        testsuites_metadata = self.root.attrib if self.root is not None else {}
        testbed = "Hardware"; sim_files = glob.glob(self.testsuite_root + "/testbed_logs/*/bringup_success/sim-config.yaml")
        if(len(sim_files) > 0): testbed = "Simulation"
        chain = "Unknown"; submission_cmd = self.run_info.get("submission_cmd", [])
        try:
            if isinstance(submission_cmd, list) and "--chain" in submission_cmd: chain_index = submission_cmd.index("--chain"); chain = submission_cmd[chain_index + 1]
        except Exception: logger.warning("Could not determine chain from run_info['submission_cmd']")
        testsuites_metadata.update({"firex_id": self.run_info.get("firex_id", "Unknown"),"group": self.run_info.get("group", "Unknown"),"lineup": self.run_info.get("inputs", {}).get("lineup", "Unknown"),"testbed": testbed,"chain": chain,"workspace": workspace,"tag": version,})
        return testsuites_metadata

    def _parse_show_version(self, file_path):
        version = ""; workspace = ""
        try:
            with open(file_path) as f:
                lines = f.readlines()
                if lines:
                    header = lines[0]; parts = header.split(",")
                    if len(parts) > 1 and len(parts[1].split(" ")) > 2: version = parts[1].split(" ")[2].strip()
                    for line in lines:
                        if line.strip().startswith("Workspace"): workspace_parts = line.split(":"); workspace = workspace_parts[1].strip() if len(workspace_parts) > 1 else ""; break
        except IOError as e: logger.warning(f"Could not read show_version file {file_path}: {e}")
        except Exception as e: logger.warning(f"Error parsing show_version file {file_path}: {e}")
        return version, workspace


    def _find_verification_origin(self, database, lineup, group, plan_id, testcase_name, start_timestamp):
        """
        Looks back through history to find the most recent run
        where the given testcase was explicitly verified (has 'verified_by').
        Returns origin run_id, timestamp, verified_by, and logs.
        """
        logger.debug(f"Starting verification origin search for TC '{testcase_name}' in {group}/{plan_id}/{lineup} starting before {start_timestamp}")
        current_ts = start_timestamp
        origin_run_id = None
        origin_timestamp = None
        origin_verified_by = None
        origin_logs = None  

        if not (hasattr(database, 'get_historical_testsuite') and callable(getattr(database, 'get_historical_testsuite'))):
             logger.error("Database object missing 'get_historical_testsuite' method in _find_verification_origin.")
             return None, None, None, None # Return four Nones

        for i in range(MAX_VERIFICATION_LOOKUP_DEPTH):
            if current_ts is None:
                 logger.warning(f"Cannot backtrack further for TC '{testcase_name}': Missing timestamp."); break
            logger.debug(f"Lookup iteration {i+1}/{MAX_VERIFICATION_LOOKUP_DEPTH}, searching before ts: {current_ts}")
            try:
                 historical_run = database.get_historical_testsuite(lineup, group, plan_id, before_timestamp=current_ts)
            except Exception as e: logger.error(f"DB error during backtracking lookup {i+1}: {e}"); break

            if not historical_run: logger.debug("No further historical run found."); break

            next_ts = historical_run.get("timestamp") # Timestamp of the run just found

            history_testcase = None
            for tc in historical_run.get("testcases", []):
                if tc.get("name") == testcase_name: history_testcase = tc; break

            if history_testcase:
                verified_by = history_testcase.get("verified_by")
                if verified_by:
                    # Found the origin verification!
                    origin_run_id = historical_run.get("run_id")
                    origin_timestamp = historical_run.get("timestamp")
                    origin_verified_by = verified_by
                    origin_logs = history_testcase.get("logs", "") # Get logs from this TC
                    logger.info(f"Found origin for TC '{testcase_name}': Run=[{origin_run_id}], VerifiedBy=[{origin_verified_by}]")
                    break # Exit loop successfully
                else:
                    logger.debug(f"Run {historical_run.get('run_id','N/A')} did not have 'verified_by' for TC '{testcase_name}'.")
            else:
                logger.warning(f"TC '{testcase_name}' not found in historical run {historical_run.get('run_id','N/A')}. Stopping.")
                break

            # Prepare for next iteration
            if not next_ts or next_ts >= current_ts:
                 logger.warning(f"Timestamp issue in run {historical_run.get('run_id','N/A')} (current='{current_ts}', next='{next_ts}'). Stopping.")
                 break
            current_ts = next_ts

        if i == MAX_VERIFICATION_LOOKUP_DEPTH - 1 and not origin_verified_by:
            logger.warning(f"Origin search reached max depth ({MAX_VERIFICATION_LOOKUP_DEPTH}) for TC '{testcase_name}'.")

        return origin_run_id, origin_timestamp, origin_verified_by, origin_logs

    def get_testsuites(self, database, run_info):
        documents = []; current_run_id = run_info.get("firex_id", "Unknown")
        if self.root is None: logger.error("XML root is None..."); return documents
        for testsuite in self.root.findall("./testsuite"):
            stats = testsuite.attrib; properties = testsuite.find("properties"); testcases = testsuite.findall("testcase")
            if properties is None: logger.warning(f"Skipping testsuite without properties..."); continue
            failures_count = int(stats.get("failures", 0)); errors_count = int(stats.get("errors", 0)); test_passed = failures_count + errors_count == 0
            current_run_timestamp = str(stats.get("timestamp", "N/A"))
            data = { "group": run_info.get("group", "Unknown"), "efr": run_info.get("tag", "Unknown"), "run_id": current_run_id, "lineup": run_info.get("lineup", "Unknown"), "tests": int(stats.get("tests", 0)), "failures": failures_count, "errors": errors_count, "disabled": int(stats.get("disabled", 0)), "skipped": int(stats.get("skipped", 0)), "timestamp" : current_run_timestamp, "health": "ok", "testcases": [], "bugs": [] }
            b4_keys = ["test.plan_id", "test.description", "test.uuid", "testsuite_hash", "testsuite_root"]
            cafy_keys_mappings = {"testsuite_name": "plan_id", "testsuite_hash": "testsuite_hash", "testsuite_root": "testsuite_root"}
            framework_property = properties.find("./property[@name='framework']"); framework = framework_property.get("value") if framework_property is not None else "unknown"
            if properties is not None:
                if framework == "cafy2":
                    for p in properties.findall("property"): prop_name = p.get("name"); data[cafy_keys_mappings[prop_name]] = p.get("value") if prop_name in cafy_keys_mappings else data.get(cafy_keys_mappings.get(prop_name))
                else:
                    for p in properties.findall("property"): prop_name = p.get("name"); data[prop_name.replace("test.", "")] = p.get("value") if prop_name in b4_keys else data.get(prop_name.replace("test.", ""))
            historial_testsuite = None; historical_timestamp = None
            plan_id_for_lookup = data.get("plan_id"); group_for_lookup = data.get("group"); lineup_for_lookup = data.get("lineup")
            if plan_id_for_lookup and group_for_lookup != "Unknown" and lineup_for_lookup != "Unknown":
                try:
                    if hasattr(database, 'get_historical_testsuite') and callable(getattr(database, 'get_historical_testsuite')):
                         historial_testsuite = database.get_historical_testsuite(lineup_for_lookup, group_for_lookup, plan_id_for_lookup)
                         if historial_testsuite:
                             retrieved_run_id = historial_testsuite.get('run_id', 'N/A'); retrieved_ts = historial_testsuite.get("timestamp", 'N/A')
                             logger.info(f"Retrieved immediate predecessor for inheritance: run_id=[{retrieved_run_id}], timestamp=[{retrieved_ts}]")
                             historical_timestamp = historial_testsuite.get("timestamp")
                         else: logger.info(f"No immediate predecessor found for {group_for_lookup}/{plan_id_for_lookup}/{lineup_for_lookup}.")
                    else: logger.warning("Database object missing 'get_historical_testsuite'.")
                except Exception as e: logger.error(f"Error calling get_historical_testsuite for {group_for_lookup}/{plan_id_for_lookup}/{lineup_for_lookup}: {e}")
            else: logger.warning(f"Skipping historical lookup due to missing keys...")
            if historial_testsuite:
                for bug in historial_testsuite.get("bugs", []):
                    name = bug.get("name"); bug_type = bug.get("type")
                    if name and bug_type:
                        try:
                            if bug_type == "DDTS": data["bugs"].append(ddts.inherit(name)); data["health"] = "unstable" if test_passed and ddts.is_open(name) else data["health"]
                            elif bug_type == "TechZone": data["bugs"].append(techzone.inherit(name))
                            elif bug_type == "Github": data["bugs"].append(github.inherit(name))
                        except Exception as e: logger.error(f"Error inheriting bug {name} of type {bug_type}: {e}")
                    else: logger.warning(f"Skipping bug inheritance due to missing name or type: {bug}")
            data["testcases"] = self._create_testsuites(database, testcases, historial_testsuite, historical_timestamp, group_for_lookup, plan_id_for_lookup, lineup_for_lookup)
            documents.append(data)
        return documents


    def _create_testsuites(self, database, testcases, historial_testsuite, historical_timestamp, group, plan_id, lineup):
        testsuites = []
        for testcase in testcases:
            current_test_name = testcase.get("name", "Unnamed Test")
            inheritance_possible = historial_testsuite is not None
            history = None # Immediate predecessor's TC data
            if inheritance_possible:
                for e in historial_testsuite.get("testcases", []):
                    if e.get("name") == current_test_name: history = e; break

            testcase_data = {"name": current_test_name, "time": float(testcase.get("time", 0))}
            failure_el = testcase.find("failure"); error_el = testcase.find("error"); skipped_el = testcase.find("skipped")
            current_status = "passed"; should_inherit_label = False; current_log = None

            if skipped_el is not None: current_status = "skipped"; testcase_data["label"] = "Test Skipped. No Label Required."
            elif error_el is not None and error_el.get("message") is None: current_status = "aborted"; should_inherit_label = bool(history and history.get("status") == "aborted")
            elif (error_el is not None and error_el.get("message")) or failure_el is not None:
                current_status = "failed"; text = error_el.text if error_el is not None else failure_el.text
                current_log = str(text).strip() if text else ""; testcase_data["message"] = "Failed"; testcase_data["logs"] = current_log
                should_inherit_label = bool(history and history.get("status") == "failed")
            else: testcase_data["label"] = "Test Passed. No Label Required."

            testcase_data["status"] = current_status
            # Initialize fields
            testcase_data["inherited_label"] = False; testcase_data["inheritance_date"] = None # Will hold ORIGIN timestamp
            testcase_data["inheritance_source_run_id"] = None # Will hold IMMEDIATE predecessor run_id
            testcase_data["inheritance_reason"] = None; testcase_data["log_similarity_score"] = None # Vs ORIGIN log now
            testcase_data["original_verification_run_id"] = None; testcase_data["verified_by"] = None

            if should_inherit_label and history: # Inherit label etc from immediate predecessor
                testcase_data["inherited_label"] = True
                testcase_data["triage_status"] = history.get("triage_status", "New")
                testcase_data["label"] = history.get("label", "")
                testcase_data["bugs"] = history.get("bugs", [])

                # --- Find TRUE Origin Verification & Logs via Backtracking ---
                origin_run_id, origin_timestamp, origin_verified_by, origin_logs = self._find_verification_origin(
                    database, lineup, group, plan_id, current_test_name, historical_timestamp # Start search before predecessor
                )

                # Store origin details
                testcase_data["original_verification_run_id"] = origin_run_id
                testcase_data["verified_by"] = origin_verified_by
                testcase_data["inheritance_date"] = origin_timestamp # Use origin timestamp here

                # Store predecessor details
                predecessor_run_id = historial_testsuite.get("run_id", "Unknown") if historial_testsuite else "Unknown"
                testcase_data["inheritance_source_run_id"] = predecessor_run_id

                # --- Update Reason String ---
                reason_label = history.get('label', '')
                if origin_run_id:
                     testcase_data["inheritance_reason"] = (f"Inherited label '{reason_label}' (verified by {origin_verified_by} in run [{origin_run_id}] on {origin_timestamp}) via predecessor run [{predecessor_run_id}].")
                else:
                     reason_timestamp = historical_timestamp # Fallback ts
                     testcase_data["inheritance_reason"] = (f"Inherited label '{reason_label}' from predecessor run [{predecessor_run_id}] on {reason_timestamp}, but original verification details not found via backtracking.")

                # --- Calculate Log Similarity (vs ORIGIN log) ---
                if current_status == "failed" and self.embedding_model:
                    if current_log and origin_logs: # Use origin_logs now
                        try:
                            logger.debug(f"Calculating log similarity for TC '{current_test_name}' vs ORIGIN log from run [{origin_run_id}]")
                            embeddings = self.embedding_model.embed_documents([current_log, origin_logs])
                            vec1 = np.array(embeddings[0]).reshape(1, -1); vec2 = np.array(embeddings[1]).reshape(1, -1)
                            similarity = cosine_similarity(vec1, vec2)[0][0]
                            testcase_data["log_similarity_score"] = float(similarity)
                            logger.debug(f"Log similarity score vs origin: {similarity:.4f}")
                        except Exception as e: logger.error(f"Failed to calculate log similarity vs origin for TC '{current_test_name}': {e}")
                    elif current_log and not origin_logs:
                        logger.debug(f"Cannot calculate log similarity for TC '{current_test_name}': Origin log is missing/empty (Run ID: {origin_run_id}).")
                    else:
                        logger.debug(f"Skipping log similarity for TC '{current_test_name}': Current log is missing.")
                elif current_status == "failed": logger.warning(f"Cannot calculate log similarity for '{current_test_name}': Embedding model not available.")
                # --- End Log Similarity ---

            elif current_status in ["failed", "aborted"]:
                 testcase_data["triage_status"] = "New"; testcase_data["label"] = ""

            # Ensure default fields are set if not populated otherwise
            testcase_data.setdefault("triage_status", "New" if current_status in ["failed", "aborted"] else None)
            testcase_data.setdefault("label", "" if not testcase_data.get("label") else testcase_data.get("label"))
            testcase_data.setdefault("bugs", [])
            testcase_data.setdefault("original_verification_run_id", None); testcase_data.setdefault("verified_by", None)
            testcase_data.setdefault("log_similarity_score", None); testcase_data.setdefault("inheritance_date", None)
            testcase_data.setdefault("inheritance_source_run_id", None); testcase_data.setdefault("inheritance_reason", None)

            testsuites.append(testcase_data)
        return testsuites