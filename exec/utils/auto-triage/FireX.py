# auto-triage/FireX.py
import json
import glob
import os
import logging
import numpy as np


from DDTS import DDTS
from TechZone import TechZone
from Github import Github
from Embeddings import Embeddings 


from sklearn.metrics.pairwise import cosine_similarity

# Get a logger instance
logger = logging.getLogger(__name__)

ddts = DDTS()
techzone = TechZone()
github = Github()

# Define a constant for max backtracking depth
MAX_VERIFICATION_LOOKUP_DEPTH = 30 

class FireX:

    def __init__(self, xml_root):
        """Track FireX files and store run info from run.json"""
        self.root = xml_root
        testsuite_root_property = self.root.find(".//properties/property[@name='testsuite_root']")
        # Add check for None before calling .get()
        if testsuite_root_property is None:
            raise ValueError("Could not find 'testsuite_root' property in xunit data.")
        self.testsuite_root = testsuite_root_property.get("value")
        if self.testsuite_root is None:
             raise ValueError("Value for 'testsuite_root' property is None in xunit data.")

        run_info_file = os.path.join(self.testsuite_root, "run.json")
        if not os.path.exists(run_info_file):
            raise FileNotFoundError(f"run.json not found at {run_info_file}")
        try:
            with open(run_info_file) as f:
                self.run_info = json.load(f)
        except json.JSONDecodeError as e:
             raise ValueError(f"Error decoding JSON from {run_info_file}: {e}") from e

        try:
            logger.info("Initializing embedding model...")
            self.embeddings = Embeddings() # Instance of your Embeddings class
            self.embedding_model = self.embeddings.get_model() # Get the actual model
            logger.info("Embedding model initialized successfully.")
        except Exception as e:
            logger.error(f"Failed to initialize embedding model: {e}", exc_info=True)
            self.embedding_model = None # Set to None if initialization fails

    def get_group(self):
        """Return group to be used for subscription checking"""
        return self.run_info.get("group", "Unknown")

    def get_run_information(self, version, workspace):
        """Extract FireX metadata to insert into Database"""
        # Only extract version and workspace if B4 Architecture, CAFY runs will result in nonempty strings
        if version == "" and workspace == "":
            show_version_files = glob.glob(self.testsuite_root + "/tests_logs/*/debug_files/dut*/show_version")
            if show_version_files:
                show_version = show_version_files[0]
                # Use helper method for parsing
                version, workspace = self._parse_show_version(show_version)
            else:
                logger.warning("show_version file not found. Could not extract version/workspace.")

        testsuites_metadata = self.root.attrib if self.root is not None else {}

        # All runs are assumed to be hardware unless a sim-config.yaml files exists indicating it is a simulation
        testbed = "Hardware"
        sim_files = glob.glob(self.testsuite_root + "/testbed_logs/*/bringup_success/sim-config.yaml")
        if(len(sim_files) > 0):
            testbed = "Simulation"

        chain = "Unknown"
        try:
            # Ensure 'submission_cmd' exists and is a list
            submission_cmd = self.run_info.get("submission_cmd", [])
            if isinstance(submission_cmd, list) and "--chain" in submission_cmd:
                 chain_index = submission_cmd.index("--chain")
                 if chain_index + 1 < len(submission_cmd):
                    chain = submission_cmd[chain_index + 1]
        except ValueError:
             logger.warning("Could not determine chain from run_info['submission_cmd']")

        # Aggregate all metadata to insert into Database
        testsuites_metadata.update({
            "firex_id": self.run_info.get("firex_id", "Unknown"), 
            "group": self.run_info.get("group", "Unknown"),
            "lineup": self.run_info.get("inputs", {}).get("lineup", "Unknown"), 
            "testbed": testbed,
            "chain": chain,
            "workspace": workspace,
            "tag": version,
        })

        return testsuites_metadata

    def _parse_show_version(self, file_path):
        """Helper to parse version and workspace from show version file."""
        version = ""
        workspace = ""
        try:
            with open(file_path) as f:
                lines = f.readlines()
                if lines: # Check if file is not empty
                    header = lines[0]
                    parts = header.split(",")
                    if len(parts) > 1 and len(parts[1].split(" ")) > 2:
                        version = parts[1].split(" ")[2].strip()

                    for line in lines:
                        if line.strip().startswith("Workspace"):
                            workspace_parts = line.split(":")
                            if len(workspace_parts) > 1:
                                workspace = workspace_parts[1].strip()
                            break # Found workspace, exit loop
        except IOError as e:
            logger.warning(f"Could not read show_version file {file_path}: {e}")
        except Exception as e: # Catch other potential errors during parsing
            logger.warning(f"Error parsing show_version file {file_path}: {e}")
        return version, workspace

    def _find_verification_origin(self, database, lineup, group, plan_id, testcase_name, start_timestamp):
        """
        Looks back through history for verification details. Prioritizes finding the
        most recent run with 'verified_by'. If not found, returns details of the
        oldest run checked during the search.
        """
        logger.debug(f"Origin search started: TC='{testcase_name}', Config={group}/{plan_id}/{lineup}, Before={start_timestamp}")
        current_ts = start_timestamp

        # Store details if we find verified_by
        found_origin_run_id = None
        found_origin_timestamp = None
        found_verified_by = None
        found_origin_logs = None

        # Store details of the oldest run examined in case verified_by is never found
        oldest_run_id_checked = None
        oldest_timestamp_checked = None

        if not (hasattr(database, 'get_historical_testsuite') and callable(getattr(database, 'get_historical_testsuite'))):
             logger.error("DB object missing 'get_historical_testsuite' in _find_verification_origin.")
             return None, None, None, None # Return four Nones

        for i in range(MAX_VERIFICATION_LOOKUP_DEPTH):
            if current_ts is None: logger.warning(f"Cannot backtrack: Missing timestamp."); break
            logger.debug(f"Lookup {i+1}/{MAX_VERIFICATION_LOOKUP_DEPTH}, searching before: {current_ts}")
            try:
                 historical_run = database.get_historical_testsuite(lineup, group, plan_id, before_timestamp=current_ts)
            except Exception as e: logger.error(f"DB error during lookup {i+1}: {e}"); break

            if not historical_run: logger.debug("No further historical run found."); break

            # Record this run as the oldest checked *so far* in this search
            # Do this *before* checking verified_by for this run
            oldest_run_id_checked = historical_run.get("run_id")
            oldest_timestamp_checked = historical_run.get("timestamp")
            logger.debug(f"Checking run {oldest_run_id_checked} @ {oldest_timestamp_checked}")

            next_ts = historical_run.get("timestamp") 

            history_testcase = None
            for tc in historical_run.get("testcases", []):
                if tc.get("name") == testcase_name: history_testcase = tc; break

            if history_testcase:
                verified_by = history_testcase.get("verified_by")
                if verified_by:
                    # Found the most recent verification! Store these details.
                    found_origin_run_id = historical_run.get("run_id")
                    found_origin_timestamp = historical_run.get("timestamp")
                    found_verified_by = verified_by
                    found_origin_logs = history_testcase.get("logs", "")
                    logger.info(f"Found verification details for '{testcase_name}': Run=[{found_origin_run_id}], By=[{found_verified_by}]")
                    # *** Keep searching backwards *** to find the oldest run where the TC exists.
                    # The details stored above (found_*) hold the LATEST verification seen.
                else:
                    logger.debug(f"Run {oldest_run_id_checked} did not have 'verified_by' for TC '{testcase_name}'.")
            else:
                logger.warning(f"TC '{testcase_name}' not found in run {oldest_run_id_checked}. Stopping search for this TC.")
                break # Stop if the test case disappears

            # Check timestamp for next iteration
            if not next_ts or next_ts >= current_ts:
                 logger.warning(f"Timestamp issue in run {oldest_run_id_checked} (current='{current_ts}', next='{next_ts}'). Stopping.")
                 break
            current_ts = next_ts # Setup for the next loop

        if i == MAX_VERIFICATION_LOOKUP_DEPTH - 1:
            logger.warning(f"Origin search reached max depth ({MAX_VERIFICATION_LOOKUP_DEPTH}) for TC '{testcase_name}'.")


        if found_verified_by:
            # If we found a verification at some point, return those details
            logger.debug(f"Returning details from verification origin run: {found_origin_run_id}")
            return found_origin_run_id, found_origin_timestamp, found_verified_by, found_origin_logs
        elif oldest_run_id_checked:
             # If no verification was ever found, return details of the oldest run checked
             logger.debug(f"No 'verified_by' found. Returning details from oldest run checked: {oldest_run_id_checked}")
             # Logs are only relevant if verified_by was found, so return None for logs here
             return oldest_run_id_checked, oldest_timestamp_checked, None, None
        else:
             # If no historical runs were found at all in the loop
             logger.debug("No historical runs were successfully checked.")
             return None, None, None, None


    def get_testsuites(self, database, run_info):
        """Gather testsuite data to store into Database"""
        documents = []
        current_run_id = run_info.get("firex_id", "Unknown")

        if self.root is None:
             logger.error("XML root is None, cannot process test suites.")
             return documents

        # Visit all testsuites within a run
        for testsuite in self.root.findall("./testsuite"):
            stats = testsuite.attrib
            properties = testsuite.find("properties")
            testcases = testsuite.findall("testcase")

            if properties is None:
                logger.warning(f"Skipping testsuite without properties element. Attributes: {stats}")
                continue

            failures_count = int(stats.get("failures", 0))
            errors_count = int(stats.get("errors", 0))
            test_passed = failures_count + errors_count == 0
            current_run_timestamp = str(stats.get("timestamp", "N/A"))

            # Initialize base data dictionary
            data = {
                "group": run_info.get("group", "Unknown"),
                "efr": run_info.get("tag", "Unknown"),
                "run_id": current_run_id,
                "lineup": run_info.get("lineup", "Unknown"),
                "tests": int(stats.get("tests", 0)),
                "failures": failures_count,
                "errors": errors_count,
                "disabled": int(stats.get("disabled", 0)),
                "skipped": int(stats.get("skipped", 0)),
                "timestamp" : current_run_timestamp,
                "health": "ok",
                "testcases": [],
                "bugs": []
            }


            b4_keys = ["test.plan_id", 
                       "test.description", 
                       "test.uuid", 
                       "testsuite_hash", 
                       "testsuite_root"]
            
            cafy_keys_mappings = {
                "testsuite_name": "plan_id", 
                "testsuite_hash": "testsuite_hash", 
                "testsuite_root": "testsuite_root"
                }
            framework_property = properties.find("./property[@name='framework']")
            framework = framework_property.get("value") if framework_property is not None else "unknown"

            if properties is not None:
                if framework == "cafy2":
                    for p in properties.findall("property"):
                        prop_name = p.get("name")
                        if prop_name in cafy_keys_mappings:
                             data[cafy_keys_mappings[prop_name]] = p.get("value")
                else: # Assume B4 or other framework
                    for p in properties.findall("property"):
                        prop_name = p.get("name")
                        if prop_name in b4_keys:
                             data[prop_name.replace("test.", "")] = p.get("value")
            


            # Grab historical testsuite if it exists
            historial_testsuite = None
            historical_timestamp = None # Immediate predecessor details
            plan_id_for_lookup = data.get("plan_id")
            group_for_lookup = data.get("group")
            lineup_for_lookup = data.get("lineup")

            if plan_id_for_lookup and group_for_lookup != "Unknown" and lineup_for_lookup != "Unknown":
                try:
                    if hasattr(database, 'get_historical_testsuite') and callable(getattr(database, 'get_historical_testsuite')):
                         historial_testsuite = database.get_historical_testsuite(lineup_for_lookup, group_for_lookup, plan_id_for_lookup) # No before_timestamp -> gets latest
                         if historial_testsuite:
                             retrieved_run_id = historial_testsuite.get('run_id', 'N/A')
                             retrieved_ts = historial_testsuite.get("timestamp", 'N/A')
                             logger.info(f"Retrieved immediate predecessor for inheritance: run_id=[{retrieved_run_id}], timestamp=[{retrieved_ts}]")
                             historical_timestamp = historial_testsuite.get("timestamp") # Timestamp of immediate predecessor
                         else:
                             logger.info(f"No immediate predecessor found for {group_for_lookup}/{plan_id_for_lookup}/{lineup_for_lookup}.")
                    else:
                         logger.warning("Database object does not have a callable 'get_historical_testsuite' method.")
                except Exception as e:
                    logger.error(f"Error calling get_historical_testsuite for {group_for_lookup}/{plan_id_for_lookup}/{lineup_for_lookup}: {e}")
            else:
                logger.warning(f"Skipping historical lookup due to missing keys: plan_id={plan_id_for_lookup}, group={group_for_lookup}, lineup={lineup_for_lookup}")

            # Inherit associated bugs from immediate predecessor only
            if historial_testsuite:
                for bug in historial_testsuite.get("bugs", []):
                    name = bug.get("name")
                    bug_type = bug.get("type")
                    if name and bug_type:
                        try:
                            if bug_type in ["DDTS", "TechZone", "Github"]:
                                data["bugs"].append(globals()[bug_type.lower()].inherit(name)) 
                                if bug_type == "DDTS" and test_passed and ddts.is_open(name):
                                    data["health"] = "unstable"
                        except Exception as e:
                            logger.error(f"Error inheriting bug {name} of type {bug_type}: {e}")
                    else:
                        logger.warning(f"Skipping bug inheritance due to missing name or type: {bug}")

            # Pass database object AND immediate predecessor details AND lookup keys to _create_testsuites
            data["testcases"] = self._create_testsuites(
                database, 
                testcases, 
                historial_testsuite,
                historical_timestamp,
                group_for_lookup,   
                plan_id_for_lookup,
                lineup_for_lookup
            )
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

            # Initialize basic fields common to all statuses
            testcase_data = {"name": current_test_name, "time": float(testcase.get("time", 0))}

            failure_el = testcase.find("failure"); error_el = testcase.find("error"); skipped_el = testcase.find("skipped")
            current_status = "passed"; should_inherit_label = False; current_log = None

            # Determine Status and Handle Status-Specific Fields

            if skipped_el is not None:
                current_status = "skipped"
                testcase_data["status"] = current_status
                testcase_data["label"] = "Test Skipped. No Label Required."
                # No other fields needed for skipped

            elif error_el is not None and error_el.get("message") is None:
                current_status = "aborted"
                testcase_data["status"] = current_status
                should_inherit_label = bool(history and history.get("status") == "aborted")

                if should_inherit_label and history: # Inherit for ABORTED
                    # Populate ALL relevant fields ONLY if inheriting
                    testcase_data["inherited_label"] = True
                    testcase_data["triage_status"] = history.get("triage_status", "New")
                    testcase_data["label"] = history.get("label", "")
                    testcase_data["bugs"] = history.get("bugs", [])

                    origin_run_id, origin_timestamp, origin_verified_by, _ = self._find_verification_origin(
                        database, lineup, group, plan_id, current_test_name, historical_timestamp
                    )
                    testcase_data["original_verification_run_id"] = origin_run_id
                    testcase_data["verified_by"] = origin_verified_by if origin_verified_by is not None else "Unknown"
                    testcase_data["inheritance_date"] = origin_timestamp

                    predecessor_run_id = historial_testsuite.get("run_id", "Unknown") if historial_testsuite else "Unknown"
                    testcase_data["inheritance_source_run_id"] = predecessor_run_id

                    # Update Reason String
                    reason_label = history.get('label', ''); reason_ts_str = f"on {origin_timestamp}" if origin_timestamp else "(ts unknown)"; reason_verified_by = testcase_data["verified_by"]
                    if origin_run_id and reason_verified_by != "Unknown": testcase_data["inheritance_reason"] = (f"Inherited label '{reason_label}' (verified by {reason_verified_by} in run [{origin_run_id}] {reason_ts_str}) via predecessor [{predecessor_run_id}].")
                    elif origin_run_id: testcase_data["inheritance_reason"] = (f"Inherited label '{reason_label}' (origin run [{origin_run_id}] {reason_ts_str}, verified_by Unknown) via predecessor [{predecessor_run_id}].")
                    else: pred_ts_str = f"on {historical_timestamp}" if historical_timestamp else "(ts unknown)"; testcase_data["inheritance_reason"] = (f"Inherited label '{reason_label}' from predecessor [{predecessor_run_id}] {pred_ts_str}, origin details not found.")

                else: # Aborted, but no inheritance (New Issue)
                    testcase_data["inherited_label"] = False
                    testcase_data["triage_status"] = "New"
                    testcase_data["label"] = "" # Needs labeling
                    testcase_data["bugs"] = []
                    # No inheritance/origin/similarity fields populated

            elif (error_el is not None and error_el.get("message")) or failure_el is not None:
                current_status = "failed"
                testcase_data["status"] = current_status
                text = error_el.text if error_el is not None else failure_el.text
                current_log = str(text).strip() if text else ""
                testcase_data["message"] = "Failed"
                testcase_data["logs"] = current_log
                should_inherit_label = bool(history and history.get("status") == "failed")

                if should_inherit_label and history: # Inherit for FAILED
                    # Populate ALL relevant fields ONLY if inheriting
                    testcase_data["inherited_label"] = True
                    testcase_data["triage_status"] = history.get("triage_status", "New")
                    testcase_data["label"] = history.get("label", "")
                    testcase_data["bugs"] = history.get("bugs", [])

                    origin_run_id, origin_timestamp, origin_verified_by, origin_logs = self._find_verification_origin(
                        database, lineup, group, plan_id, current_test_name, historical_timestamp
                    )
                    testcase_data["original_verification_run_id"] = origin_run_id
                    testcase_data["verified_by"] = origin_verified_by if origin_verified_by is not None else "Unknown"
                    testcase_data["inheritance_date"] = origin_timestamp

                    predecessor_run_id = historial_testsuite.get("run_id", "Unknown") if historial_testsuite else "Unknown"
                    testcase_data["inheritance_source_run_id"] = predecessor_run_id

                    # Update Reason String
                    reason_label = history.get('label', ''); reason_ts_str = f"on {origin_timestamp}" if origin_timestamp else "(ts unknown)"; reason_verified_by = testcase_data["verified_by"]
                    if origin_run_id and reason_verified_by != "Unknown": testcase_data["inheritance_reason"] = (f"Inherited label '{reason_label}' (verified by {reason_verified_by} in run [{origin_run_id}] {reason_ts_str}) via predecessor [{predecessor_run_id}].")
                    elif origin_run_id: testcase_data["inheritance_reason"] = (f"Inherited label '{reason_label}' (origin run [{origin_run_id}] {reason_ts_str}, verified_by Unknown) via predecessor [{predecessor_run_id}].")
                    else: pred_ts_str = f"on {historical_timestamp}" if historical_timestamp else "(ts unknown)"; testcase_data["inheritance_reason"] = (f"Inherited label '{reason_label}' from predecessor [{predecessor_run_id}] {pred_ts_str}, origin details not found.")

                    # Calculate Log Similarity (vs ORIGIN log if available, else predecessor)
                    # Initialize score to None ONLY when inheriting
                    testcase_data["log_similarity_score"] = None
                    similarity_calculated = False
                    if self.embedding_model:
                        if current_log and origin_logs:
                            try:
                                logger.debug(f"Calculating log similarity for TC '{current_test_name}' vs ORIGIN log from run [{origin_run_id}]")
                                embeddings = self.embedding_model.embed_documents([current_log, origin_logs])
                                vec1=np.array(embeddings[0]).reshape(1, -1); vec2=np.array(embeddings[1]).reshape(1, -1)
                                similarity = cosine_similarity(vec1, vec2)[0][0]
                                testcase_data["log_similarity_score"] = float(similarity)
                                similarity_calculated = True
                                logger.debug(f"Log similarity score vs origin: {similarity:.4f}")
                            except Exception as e: logger.error(f"Failed similarity vs origin for TC '{current_test_name}': {e}")
                        elif current_log: # Fallback: Compare to predecessor if origin logs missing
                            predecessor_log = history.get("logs", "")
                            if predecessor_log:
                                logger.debug(f"Origin log missing for '{current_test_name}'. Comparing vs predecessor [{predecessor_run_id}].")
                                try:
                                     embeddings = self.embedding_model.embed_documents([current_log, predecessor_log])
                                     vec1=np.array(embeddings[0]).reshape(1, -1); vec2=np.array(embeddings[1]).reshape(1, -1)
                                     similarity = cosine_similarity(vec1, vec2)[0][0]
                                     testcase_data["log_similarity_score"] = float(similarity)
                                     similarity_calculated = True
                                     logger.debug(f"Log similarity score vs predecessor (fallback): {similarity:.4f}")
                                except Exception as e: logger.error(f"Failed similarity vs predecessor for TC '{current_test_name}': {e}")

                    # If similarity calculation was not successful for any reason, set to the string
                    if not similarity_calculated:
                         testcase_data["log_similarity_score"] = "no logs provided for comparison" # Changed string slightly
                elif not self.embedding_model:
                    logger.warning(f"Cannot calculate log similarity for '{current_test_name}': Model unavailable.")
                    testcase_data["log_similarity_score"] = "model unavailable"
                # --- End Log Similarity ---

                else: # Failed, but no inheritance (New Issue)
                    testcase_data["inherited_label"] = False
                    testcase_data["triage_status"] = "New"
                    testcase_data["label"] = "" # Needs labeling
                    testcase_data["bugs"] = []
                    # NOT add log_similarity_score or other inheritance/origin fields here

            else: # Passed
                current_status = "passed"
                testcase_data["status"] = current_status
                testcase_data["label"] = "Test Passed. No Label Required."
                # No other fields needed for passed

            testsuites.append(testcase_data)
        return testsuites