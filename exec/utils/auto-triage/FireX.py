# auto-triage/FireX.py
import json
import glob
import os
import logging
import numpy as np
# --- NEW: Import the FailedTestCaseCodeCollector class ---
from FailedTestCaseCodeCollector import FailedTestCaseCodeCollector

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
# Constants like NEARBY_LINE_BUFFER and LOG_CONTEXT_LINES are now managed by FailedTestCaseCodeCollector

class FireX:
    # The _log_location_regex is now part of FailedTestCaseCodeCollector

    def __init__(self, xml_root):
        """Track FireX files and store run info from run.json"""
        self.root = xml_root
        testsuite_root_property = self.root.find(".//properties/property[@name='testsuite_root']")
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
            logger.info("Initializing embedding model...") # MODIFIED: print to logger
            self.embeddings = Embeddings() # Instance of your Embeddings class
            self.embedding_model = self.embeddings.get_model() # Get the actual model
            logger.info("Embedding model initialized successfully.") # MODIFIED: print to logger
        except Exception as e:
            logger.error(f"Failed to initialize embedding model: {e}", exc_info=True)
            self.embedding_model = None 

        # --- NEW: Initialize FailedTestCaseCodeCollector ---
        self.code_collector = FailedTestCaseCodeCollector(logger_instance=logger)
        # --- End NEW ---

    def get_group(self):
        """Return group to be used for subscription checking"""
        return self.run_info.get("group", "Unknown")

    def get_run_information(self, version, workspace):
        """Extract FireX metadata to insert into Database"""
        if version == "" and workspace == "":
            show_version_files = glob.glob(self.testsuite_root + "/tests_logs/*/debug_files/dut*/show_version")
            if show_version_files:
                show_version = show_version_files[0]
                version, workspace = self._parse_show_version(show_version)
            else:
                logger.info("show_version file not found. Could not extract version/workspace.") # MODIFIED

        testsuites_metadata = self.root.attrib if self.root is not None else {}
        testbed = "Hardware"
        if glob.glob(self.testsuite_root + "/testbed_logs/*/bringup_success/sim-config.yaml"): # Simplified if
            testbed = "Simulation"

        chain = "Unknown"
        try:
            submission_cmd = self.run_info.get("submission_cmd", [])
            if isinstance(submission_cmd, list) and "--chain" in submission_cmd:
                 chain_index = submission_cmd.index("--chain")
                 if chain_index + 1 < len(submission_cmd):
                    chain = submission_cmd[chain_index + 1]
        except ValueError: # This typically won't be raised by list.index if element not found, but kept for safety
             logger.info("Could not determine chain from run_info['submission_cmd']") # MODIFIED

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
        version, workspace = "", ""
        try:
            with open(file_path) as f:
                lines = f.readlines()
                if lines: 
                    header = lines[0]
                    parts = header.split(",")
                    if len(parts) > 1 and len(parts[1].split(" ")) > 2:
                        version = parts[1].split(" ")[2].strip()
                    for line in lines:
                        if line.strip().startswith("Workspace"):
                            workspace_parts = line.split(":")
                            if len(workspace_parts) > 1: workspace = workspace_parts[1].strip()
                            break 
        except IOError as e:
            logger.warning(f"Could not read show_version file {file_path}: {e}") # MODIFIED
        except Exception as e: 
            logger.warning(f"Error parsing show_version file {file_path}: {e}") # MODIFIED
        return version, workspace

    def _find_verification_origin(self, database, lineup, group, plan_id, testcase_name, start_timestamp):
        logger.info(f"Origin search started: TC='{testcase_name}', Config={group}/{plan_id}/{lineup}, Before={start_timestamp}") # MODIFIED
        current_ts = start_timestamp
        found_origin_run_id, found_origin_timestamp, found_verified_by, found_origin_logs = None, None, None, None
        oldest_run_id_checked, oldest_timestamp_checked = None, None

        if not (hasattr(database, 'get_historical_testsuite') and callable(getattr(database, 'get_historical_testsuite'))):
             logger.error("DB object missing 'get_historical_testsuite' in _find_verification_origin.")
             return None, None, None, None 

        # Initialize i before the loop if it's checked after the loop
        # This is good practice if the loop might not run but 'i' is checked later.
        # However, in this specific logic, 'i' is only used if the loop runs at least once.
        # If the loop doesn't run, then `MAX_VERIFICATION_LOOKUP_DEPTH - 1` check won't be hit.
        # Let's assume the loop runs at least once if we reach this part or the condition is handled.
        i = 0 # Initialize i
        for i in range(MAX_VERIFICATION_LOOKUP_DEPTH):
            if current_ts is None: logger.info("Cannot backtrack: Missing timestamp."); break # MODIFIED
            logger.info(f"Lookup {i+1}/{MAX_VERIFICATION_LOOKUP_DEPTH}, searching before: {current_ts}") # MODIFIED
            try:
                 historical_run = database.get_historical_testsuite(lineup, group, plan_id, before_timestamp=current_ts)
            except Exception as e: logger.error(f"DB error during lookup {i+1}: {e}"); break
            if not historical_run: logger.info("No further historical run found."); break # MODIFIED
            oldest_run_id_checked = historical_run.get("run_id")
            oldest_timestamp_checked = historical_run.get("timestamp")
            logger.info(f"Checking run {oldest_run_id_checked} @ {oldest_timestamp_checked}") # MODIFIED
            next_ts = historical_run.get("timestamp") 
            history_testcase = next((tc for tc in historical_run.get("testcases", []) if tc.get("name") == testcase_name), None)
            if history_testcase:
                verified_by = history_testcase.get("verified_by")
                if verified_by:
                    found_origin_run_id = historical_run.get("run_id")
                    found_origin_timestamp = historical_run.get("timestamp")
                    found_verified_by = verified_by
                    found_origin_logs = history_testcase.get("logs", "")
                    logger.info(f"Found verification details for '{testcase_name}': Run=[{found_origin_run_id}], By=[{found_verified_by}]") # MODIFIED
                else:
                    logger.info(f"Run {oldest_run_id_checked} did not have 'verified_by' for TC '{testcase_name}'.") # MODIFIED
            else:
                logger.info(f"TC '{testcase_name}' not found in run {oldest_run_id_checked}. Stopping search for this TC.") # MODIFIED
                break 
            if not next_ts or next_ts >= current_ts:
                 logger.info(f"Timestamp issue in run {oldest_run_id_checked} (current='{current_ts}', next='{next_ts}'). Stopping.") # MODIFIED
                 break
            current_ts = next_ts 
        if i == MAX_VERIFICATION_LOOKUP_DEPTH - 1: # 'i' will be defined here due to initialization
            logger.info(f"Origin search reached max depth ({MAX_VERIFICATION_LOOKUP_DEPTH}) for TC '{testcase_name}'.") # MODIFIED
        if found_verified_by:
            logger.info(f"Returning details from verification origin run: {found_origin_run_id}") # MODIFIED
            return found_origin_run_id, found_origin_timestamp, found_verified_by, found_origin_logs
        elif oldest_run_id_checked:
             logger.info(f"No 'verified_by' found. Returning details from oldest run checked: {oldest_run_id_checked}") # MODIFIED
             return oldest_run_id_checked, oldest_timestamp_checked, None, None
        else:
             logger.info("No historical runs were successfully checked.") # MODIFIED
             return None, None, None, None
         
    def _is_ai_generated_label(self, testcase):
        if testcase.get("generated", False): return True
        if testcase.get("generated_label_details") is not None: return True
        if testcase.get("generated_score") is not None: return True
        if testcase.get("recalibration_candidate", False): return True
        label = testcase.get("label", "")
        if "ai suggested" in label.lower(): return True
        if label and testcase.get("triage_status") == "New": return True
        if "user_verified_label" in testcase and not testcase.get("user_verified_label"): return True
        return False

    def get_testsuites(self, database, run_info):
        logger.info(f"Starting get_testsuites for run_id: {run_info.get('firex_id', 'Unknown')}") # MODIFIED
        documents = []
        current_run_id = run_info.get("firex_id", "Unknown")
        if self.root is None:
            logger.error("XML root is None, cannot process test suites.")
            return documents

        logger.info("Processing testsuites from XML root") # MODIFIED
        for testsuite_xml_el in self.root.findall("./testsuite"): 
            stats = testsuite_xml_el.attrib
            properties = testsuite_xml_el.find("properties")
            testcases_xml_list = testsuite_xml_el.findall("testcase") 

            if properties is None:
                logger.info(f"Skipping testsuite without properties element. Attributes: {stats}") # MODIFIED
                continue

            failures_count = int(stats.get("failures", 0))
            errors_count = int(stats.get("errors", 0))
            test_passed = failures_count + errors_count == 0
            current_run_timestamp = str(stats.get("timestamp", "N/A"))
            logger.info(f"Processing testsuite with timestamp: {current_run_timestamp}, failures: {failures_count}, errors: {errors_count}") # MODIFIED

            data = {
                "group": run_info.get("group", "Unknown"),
                "efr": run_info.get("efr", run_info.get("tag", "Unknown")), 
                "run_id": current_run_id,
                "lineup": run_info.get("lineup", "Unknown"),
                "tests": int(stats.get("tests", 0)), "failures": failures_count, "errors": errors_count,
                "disabled": int(stats.get("disabled", 0)), "skipped": int(stats.get("skipped", 0)),
                "timestamp" : current_run_timestamp, "health": "ok",
                "testcases": [], "bugs": [],
                "script_full_path": "" # Initialize
            }
            logger.info(f"Initialized base data: group={data['group']}, lineup={data['lineup']}, run_id={data['run_id']}, efr={data['efr']}") # MODIFIED

            b4_keys = ["test.plan_id", "test.description", "test.uuid", "testsuite_hash", "testsuite_root"]
            cafy_keys_mappings = {"testsuite_name": "plan_id", "testsuite_hash": "testsuite_hash", "testsuite_root": "testsuite_root"}
            
            framework_property = properties.find("./property[@name='framework']")
            framework = framework_property.get("value") if framework_property is not None else "unknown"
            logger.info(f"Testsuite framework: {framework}") # MODIFIED

            for p in properties.findall("property"):
                prop_name = p.get("name")
                prop_value = p.get("value")
                if framework == "cafy2":
                    if prop_name in cafy_keys_mappings:
                        data[cafy_keys_mappings[prop_name]] = prop_value
                        logger.info(f"Set CAFY2 {cafy_keys_mappings[prop_name]}={prop_value}") # MODIFIED
                else: 
                    if prop_name in b4_keys:
                        data[prop_name.replace("test.", "")] = prop_value
                        logger.info(f"Set B4 {prop_name.replace('test.', '')}={prop_value}") # MODIFIED
                if prop_name == "script_full_path": 
                    data["script_full_path"] = prop_value
                    logger.info(f"Set script_full_path from property: {prop_value}") # MODIFIED
            
            historial_testsuite, historical_timestamp = None, None
            plan_id_for_lookup = data.get("plan_id")
            group_for_lookup = data.get("group")
            lineup_for_lookup = data.get("lineup")
            logger.info(f"Historical lookup parameters: plan_id={plan_id_for_lookup}, group={group_for_lookup}, lineup={lineup_for_lookup}") # MODIFIED
            
            if group_for_lookup == "b4-featureprofiles" and lineup_for_lookup != "Unknown":
                try:
                    logger.info(f"Attempting special lookup for b4-featureprofiles group with lineup: {lineup_for_lookup}") # MODIFIED
                    if hasattr(database, 'get_historical_testsuite') and callable(getattr(database, 'get_historical_testsuite')):
                        historial_testsuite = database.get_historical_testsuite(lineup_for_lookup, group_for_lookup, plan_id_for_lookup)
                        if historial_testsuite:
                            retrieved_run_id = historial_testsuite.get('run_id', 'N/A')
                            retrieved_ts = historial_testsuite.get("timestamp", 'N/A')
                            logger.info(f"SUCCESS: Retrieved predecessor for b4-featureprofiles: run_id=[{retrieved_run_id}], timestamp=[{retrieved_ts}]") # MODIFIED
                            historical_timestamp = historial_testsuite.get("timestamp")
                        else:
                            logger.info(f"No predecessor found for special b4-featureprofiles lookup with {group_for_lookup}/{lineup_for_lookup}") # MODIFIED
                    else:
                        logger.info("Database object does not have a callable 'get_historical_testsuite' method.") # MODIFIED
                except Exception as e:
                    logger.error(f"Error in special lookup for b4-featureprofiles: {e}", exc_info=True)
            elif plan_id_for_lookup and group_for_lookup != "Unknown" and lineup_for_lookup != "Unknown":
                try:
                    logger.info(f"Attempting standard historical lookup with plan_id={plan_id_for_lookup}") # MODIFIED
                    if hasattr(database, 'get_historical_testsuite') and callable(getattr(database, 'get_historical_testsuite')):
                        historial_testsuite = database.get_historical_testsuite(lineup_for_lookup, group_for_lookup, plan_id_for_lookup)
                        if historial_testsuite:
                            retrieved_run_id = historial_testsuite.get('run_id', 'N/A')
                            retrieved_ts = historial_testsuite.get("timestamp", 'N/A')
                            logger.info(f"SUCCESS: Retrieved immediate predecessor for inheritance: run_id=[{retrieved_run_id}], timestamp=[{retrieved_ts}]") # MODIFIED
                            historical_timestamp = historial_testsuite.get("timestamp") 
                        else:
                            logger.info(f"No immediate predecessor found for {group_for_lookup}/{plan_id_for_lookup}/{lineup_for_lookup}.") # MODIFIED
                    else:
                        logger.info("Database object does not have a callable 'get_historical_testsuite' method.") # MODIFIED
                except Exception as e:
                    logger.error(f"Error calling get_historical_testsuite for {group_for_lookup}/{plan_id_for_lookup}/{lineup_for_lookup}: {e}", exc_info=True)
            else:
                logger.info(f"Skipping historical lookup due to missing keys: plan_id={plan_id_for_lookup}, group={group_for_lookup}, lineup={lineup_for_lookup}") # MODIFIED

            ai_generated_testcases = {}
            if historial_testsuite:
                logger.info("Checking historical test cases for AI-generated indicators") # MODIFIED
                ai_generated_count = 0
                for historical_tc in historial_testsuite.get("testcases", []):
                    tc_name = historical_tc.get("name", "")
                    is_ai_generated = self._is_ai_generated_label(historical_tc)
                    ai_generated_testcases[tc_name] = is_ai_generated
                    if is_ai_generated:
                        ai_generated_count += 1
                        logger.info(f"Found AI-generated test case in historical data: {tc_name}") # MODIFIED
                if ai_generated_count > 0:
                    logger.info(f"Found {ai_generated_count} AI-generated test cases in historical data") # MODIFIED

            if historial_testsuite:
                logger.info("Processing bug inheritance from historical testsuite") # MODIFIED
                for bug in historial_testsuite.get("bugs", []):
                    name = bug.get("name")
                    bug_type = bug.get("type")
                    if name and bug_type:
                        try:
                            logger.info(f"Inheriting bug {name} of type {bug_type}") # MODIFIED
                            if bug_type.lower() in ["ddts", "techzone", "github"]:
                                class_mapping = {"ddts": "ddts", "techzone": "techzone", "github": "github"}
                                module_name = class_mapping.get(bug_type.lower())
                                if module_name:
                                    inherited_bug = globals()[module_name].inherit(name)
                                    if inherited_bug is not None:
                                        data["bugs"].append(inherited_bug)
                                        logger.info(f"Successfully inherited bug {name}") # MODIFIED
                                        if bug_type.lower() == "ddts" and test_passed and ddts.is_open(name):
                                            data["health"] = "unstable"
                                            logger.info(f"Set health to 'unstable' due to open DDTS bug {name}") # MODIFIED
                                else: # Should not happen
                                    logger.info(f"Unknown bug type '{bug_type}' for bug '{name}'") # MODIFIED
                        except Exception as e:
                            logger.error(f"Error inheriting bug {name} of type {bug_type}: {e}", exc_info=True)
                    else:
                        logger.info(f"Skipping bug inheritance due to missing name or type: {bug}") # MODIFIED
            
            if test_passed and data["bugs"]:
                logger.info(f"Testsuite passed for run_id {current_run_id}. Clearing {len(data['bugs'])} inherited bugs.") # MODIFIED
                data["bugs"] = [] 
                data["health"] = "ok" 

            logger.info(f"Calling _create_testsuites with {len(testcases_xml_list)} test cases") # MODIFIED
            data["testcases"] = self._create_testsuites(
                database,
                testcases_xml_list,
                historial_testsuite,
                historical_timestamp,
                group_for_lookup,
                plan_id_for_lookup,
                lineup_for_lookup,
                ai_generated_testcases
            )
            
            logger.info(f"Processed {len(data['testcases'])} test cases for this testsuite.") # MODIFIED
            
            # If script_full_path is still empty, try to derive it from failed test cases
            if not data["script_full_path"] and data["testcases"]:
                logger.info("Attempting to derive script_full_path from failed test cases")
                script_path = self.code_collector.get_script_path_from_testcases(data["testcases"])
                if script_path:
                    data["script_full_path"] = script_path
                    logger.info(f"Set script_full_path to: {script_path}")
            
            documents.append(data)
        
        logger.info(f"Completed get_testsuites with {len(documents)} documents") # MODIFIED
        return documents
    
    def _validate_log_path_for_testsuite(self, path, testsuite_name, testsuite_hash):
        if not path:
            logger.warning("Cannot validate empty path") # MODIFIED
            return False
        logger.info(f"Validating path '{path}' for testsuite '{testsuite_name}' with hash '{testsuite_hash}'") # MODIFIED
        if testsuite_name and testsuite_name in path:
            logger.info(f"Path contains testsuite name '{testsuite_name}'") # MODIFIED
            return True
        if testsuite_hash and testsuite_hash in path:
            logger.info(f"Path contains testsuite hash '{testsuite_hash}'") # MODIFIED
            return True
        name_variants = []
        if testsuite_name:
            name_variants = [
                testsuite_name, testsuite_name.replace(".", "_"),
                testsuite_name.replace("-", "_"), testsuite_name.replace("-", "_").replace(".", "_")
            ]
            if "." in testsuite_name:
                base_name = testsuite_name.split(".")[0]
                name_variants.extend([base_name, base_name.replace("-", "_")])
            if "-" in testsuite_name and "." in testsuite_name:
                parts = testsuite_name.split("-")
                if len(parts) > 1:
                    prefix, version_part = parts[0], parts[1].replace(".", "_")
                    variant = f"{prefix}_{version_part}"
                    if variant not in name_variants: name_variants.append(variant)
        name_variants = list(set(name_variants))
        logger.info(f"Testing path against variants: {name_variants}") # MODIFIED
        for variant in name_variants:
            if variant and variant in path:
                logger.info(f"Path contains variant '{variant}'") # MODIFIED
                return True
        if "_" in path and testsuite_name:
            for part in path.split("/"):
                if "_" in part:
                    part_segments = part.split("_")
                    if len(part_segments) >= 2 and testsuite_name.startswith(part_segments[0]):
                        logger.info(f"Path contains matching prefix '{part_segments[0]}' from testsuite name '{testsuite_name}'") # MODIFIED
                        return True
        logger.info(f"Path does not match any variant of testsuite '{testsuite_name}' or hash '{testsuite_hash}'") # MODIFIED
        return False

    # --- MODIFIED: Signature changed - removed all_raw_failure_data_for_testsuite ---
    def _create_testsuites(self, database, testcases_xml_list, historial_testsuite, historical_timestamp, group, plan_id, lineup, ai_generated_testcases=None):
        logger.info(f"Starting _create_testsuites with {len(testcases_xml_list)} testcases, plan_id={plan_id}") # MODIFIED
        if ai_generated_testcases is None: ai_generated_testcases = {}
        
        testcase_mapping = {}
        all_xml_testsuites_for_mapping = self.root.findall(".//testsuite")
        logger.info(f"Found {len(all_xml_testsuites_for_mapping)} testsuite elements in XML for context mapping.") # MODIFIED
        plan_id_testsuite_map = {}
        
        for ts_xml_el in all_xml_testsuites_for_mapping:
            ts_properties = ts_xml_el.find("properties")
            if ts_properties is None: logger.info("Skipping a testsuite element (no properties)"); continue # MODIFIED
            ts_plan_id = next((p.get("value") for name in ["test.plan_id", "testsuite_name", "plan_id"] if (p := ts_properties.find(f"./property[@name='{name}']")) is not None), None)
            if ts_plan_id is None: logger.info("Skipping a testsuite element (no plan_id)"); continue # MODIFIED
            ts_hash_prop = ts_properties.find("./property[@name='testsuite_hash']")
            ts_hash = ts_hash_prop.get("value") if ts_hash_prop is not None else None
            logger.info(f"Mapping context for testsuite element: plan_id={ts_plan_id}, hash={ts_hash}") # MODIFIED
            ts_props_dict = {p.get("name"): p.get("value") for p in ts_properties.findall("./property") if p.get("name") and p.get("value")}
            testsuite_details = {"plan_id": ts_plan_id, "testsuite_hash": ts_hash, "properties": ts_props_dict}
            plan_id_testsuite_map[ts_plan_id] = testsuite_details
            for tc_xml_el_map in ts_xml_el.findall("testcase"):
                tc_name_map = tc_xml_el_map.get("name")
                if tc_name_map:
                    testcase_mapping[tc_name_map] = dict(testsuite_details)
                    logger.info(f"Mapped TC '{tc_name_map}' to testsuite plan_id={ts_plan_id}") # MODIFIED
        
        processed_tc_list = [] 
        for testcase_xml_el in testcases_xml_list:
            current_test_name = testcase_xml_el.get("name", "Unnamed Test")
            logger.info(f"Processing testcase: {current_test_name}") # MODIFIED
            tc_props_context = testcase_mapping.get(current_test_name)
            if tc_props_context is None and plan_id in plan_id_testsuite_map:
                tc_props_context = plan_id_testsuite_map[plan_id]
                logger.info(f"No direct mapping for {current_test_name}, using overall plan_id={plan_id} context.") # MODIFIED
            
            if tc_props_context is None:
                logger.warning(f"No testsuite context found for TC {current_test_name}. Using defaults derived from main testsuite.") # MODIFIED
                tc_plan_id_for_tc, tc_hash_for_tc, tc_properties_for_tc = plan_id, None, {}
            else:
                tc_plan_id_for_tc = tc_props_context["plan_id"]
                tc_hash_for_tc = tc_props_context["testsuite_hash"]
                tc_properties_for_tc = tc_props_context["properties"]
                if current_test_name == "dummy" and tc_plan_id_for_tc != plan_id and plan_id in plan_id_testsuite_map:
                    logger.warning(f"Correcting dummy TC context from {tc_plan_id_for_tc} to main {plan_id}") # MODIFIED
                    correct_context = plan_id_testsuite_map[plan_id]
                    tc_plan_id_for_tc, tc_hash_for_tc, tc_properties_for_tc = correct_context["plan_id"], correct_context["testsuite_hash"], correct_context["properties"]
            
            logger.info(f"Using context for {current_test_name}: plan_id={tc_plan_id_for_tc}, hash={tc_hash_for_tc}") # MODIFIED
            if tc_plan_id_for_tc != plan_id:
                 logger.warning(f"TC {current_test_name} context plan_id ({tc_plan_id_for_tc}) differs from main processing plan_id ({plan_id}).") # MODIFIED
                 if plan_id in plan_id_testsuite_map: # Attempt to use main plan_id context if different
                    logger.info(f"Re-aligning TC context to main plan_id: {plan_id}")
                    correct_context = plan_id_testsuite_map[plan_id]
                    tc_plan_id_for_tc, tc_hash_for_tc, tc_properties_for_tc = correct_context["plan_id"], correct_context["testsuite_hash"], correct_context["properties"]

            root_path = tc_properties_for_tc.get("testsuite_root")
            test_log_dir = tc_properties_for_tc.get("test_log_directory")
            log_path_prop = tc_properties_for_tc.get("log") 
            logger.info(f"For {current_test_name}: root_path={root_path}, test_log_dir={test_log_dir}, log_path_prop={log_path_prop}") # MODIFIED
            
            is_ai_generated = ai_generated_testcases.get(current_test_name, False)
            if is_ai_generated: logger.info(f"SKIPPING INHERITANCE for AI-generated TC: {current_test_name}") # MODIFIED
            inheritance_possible = historial_testsuite is not None and not is_ai_generated
            history = None
            if inheritance_possible:
                logger.info(f"Checking for historical data for TC: {current_test_name}") # MODIFIED
                historical_plan_id = historial_testsuite.get("plan_id")
                historical_hash = historial_testsuite.get("testsuite_hash")
                logger.info(f"Historical testsuite: plan_id={historical_plan_id}, hash={historical_hash}") # MODIFIED
                logger.info(f"Current TC context: plan_id={tc_plan_id_for_tc}, hash={tc_hash_for_tc}") # MODIFIED
                if tc_plan_id_for_tc == historical_plan_id or tc_hash_for_tc == historical_hash:
                    logger.info(f"Contexts match, looking for matching TC in history.") # MODIFIED
                    history = next((e for e in historial_testsuite.get("testcases", []) if e.get("name") == current_test_name and not self._is_ai_generated_label(e)), None)
                else: logger.warning(f"Contexts DON'T match for {current_test_name} - not inheriting. Current: {tc_plan_id_for_tc}/{tc_hash_for_tc} vs Hist: {historical_plan_id}/{historical_hash}") # MODIFIED
                if history is None: logger.info(f"No applicable history found for TC: {current_test_name}") # MODIFIED
                else: logger.info(f"Historical data for {current_test_name} - status: {history.get('status')}, label: {history.get('label', 'None')}") # MODIFIED
            
            testcase_data_dict = {"name": current_test_name, "time": float(testcase_xml_el.get("time", 0))}
            # --- NEW: Initialize failed_code_path for each testcase ---
            testcase_data_dict["failed_code_path"] = []
            # --- End NEW ---

            failure_el = testcase_xml_el.find("failure")
            error_el = testcase_xml_el.find("error")
            skipped_el = testcase_xml_el.find("skipped")
            current_status = "passed"
            current_log = None

            if skipped_el is not None:
                current_status = "skipped"
                logger.info(f"Testcase {current_test_name} is SKIPPED") # MODIFIED
                testcase_data_dict["status"] = current_status
                testcase_data_dict["label"] = "Test Skipped. No Label Required."
            elif error_el is not None or failure_el is not None: 
                element_with_log = error_el if error_el is not None else failure_el
                is_aborted = error_el is not None and element_with_log.get("message") is None
                current_status = "aborted" if is_aborted else "failed"
                logger.info(f"Testcase {current_test_name} is {current_status.upper()}") # MODIFIED
                testcase_data_dict["status"] = current_status
                current_log = str(element_with_log.text).strip() if element_with_log.text else ""
                
                # Your extensive log extraction from files logic for current_log
                if not current_log and root_path:
                    log_file_to_check = None
                    if test_log_dir and self._validate_log_path_for_testsuite(test_log_dir, tc_plan_id_for_tc, tc_hash_for_tc):
                        log_file_to_check = os.path.join(root_path, test_log_dir, "script_console_output.txt")
                        logger.info(f"Prioritizing log from test_log_dir: {log_file_to_check}") # MODIFIED
                    elif log_path_prop and self._validate_log_path_for_testsuite(log_path_prop, tc_plan_id_for_tc, tc_hash_for_tc):
                        log_file_to_check = os.path.join(root_path, log_path_prop)
                        logger.info(f"Using log from log_path_prop: {log_file_to_check}") # MODIFIED
                    # Add other fallbacks here if they existed in your original code
                    # e.g., hash-based path construction or 'find' command

                    if log_file_to_check:
                        if os.path.exists(log_file_to_check):
                            try:
                                with os.popen(f"tail -n 100 {log_file_to_check}") as pipe: 
                                    current_log = pipe.read()
                                if current_log: logger.info(f"Extracted {len(current_log)} chars from {log_file_to_check}") # MODIFIED
                                else: logger.info(f"Tail returned empty from {log_file_to_check}") # MODIFIED
                            except Exception as e_log: logger.error(f"Error reading {log_file_to_check}: {e_log}")
                        else: logger.info(f"Log file not found: {log_file_to_check}") # MODIFIED
                    else: logger.info(f"No valid log file path determined for {current_test_name}") # MODIFIED

                testcase_data_dict["message"] = "Aborted" if current_status == "aborted" else "Failed"
                testcase_data_dict["logs"] = current_log 
                logger.info(f"Set {len(current_log) if current_log else 0} chars of logs for {current_test_name}")

                # --- NEW: Process failures for THIS testcase ---
                if current_log: 
                    tc_specific_raw_data = []
                    raw_locations = self.code_collector.extract_raw_locations(current_log) 
                    if raw_locations:
                        logger.info(f"Found {len(raw_locations)} raw file:line entries in log for {current_test_name}")
                        for fname, lnum in raw_locations:
                            tc_specific_raw_data.append((fname, lnum, current_log)) 
                    
                    testcase_data_dict["failed_code_path"] = self.code_collector.process_testsuite_failures(
                        tc_specific_raw_data
                    )
                    if testcase_data_dict["failed_code_path"]:
                        logger.info(f"Populated 'failed_code_path' for {current_test_name} with {len(testcase_data_dict['failed_code_path'])} entries.")
                # --- End NEW ---

                testcase_data_dict["inherited_label"] = False 
                should_inherit = False
                if history: 
                    logger.info(f"Checking inheritance criteria for {current_status.upper()} TC {current_test_name}") # MODIFIED
                    if history.get("label_id") and history.get("label","").strip(): should_inherit = True; logger.info("Will inherit by label_id") # MODIFIED
                    elif history.get("status") == current_status and history.get("label","").strip(): should_inherit = True; logger.info("Will inherit by status") # MODIFIED
                    else: logger.info(f"No inheritance criteria met for {current_test_name}") # MODIFIED
                
                if should_inherit:
                    logger.info(f"INHERITING LABEL for {current_test_name}") # MODIFIED
                    try:
                        testcase_data_dict.update({
                            "inherited_label": True, "triage_status": "Resolved",
                            "label": history.get("label", ""), "bugs": history.get("bugs", []),
                            "label_id": history.get("label_id", "")})
                        logger.info(f"Set inherited label: '{testcase_data_dict['label']}' for {current_test_name}") # MODIFIED
                        origin_run_id, origin_ts, origin_vb, origin_logs = self._find_verification_origin(
                            database, lineup, group, tc_plan_id_for_tc, current_test_name, historical_timestamp)
                        logger.info(f"Found verification origin: run_id={origin_run_id}, verified_by={origin_vb}") # MODIFIED
                        testcase_data_dict.update({
                            "original_verification_run_id": origin_run_id,
                            "verified_by": origin_vb if origin_vb is not None else "Unknown",
                            "inheritance_date": origin_ts,
                            "inheritance_source_run_id": historial_testsuite.get("run_id", "Unknown")})
                        
                        # Your existing inheritance_reason logic
                        reason_label = history.get('label', '')
                        reason_ts_str = f"on {origin_ts}" if origin_ts else "(ts unknown)"
                        reason_verified_by = testcase_data_dict["verified_by"]
                        if origin_run_id and reason_verified_by != "Unknown":
                            inheritance_reason = f"Inherited label '{reason_label}' (verified by {reason_verified_by} in run [{origin_run_id}] {reason_ts_str}) via predecessor [{testcase_data_dict['inheritance_source_run_id']}]."
                        elif origin_run_id:
                            inheritance_reason = f"Inherited label '{reason_label}' (origin run [{origin_run_id}] {reason_ts_str}, verified_by Unknown) via predecessor [{testcase_data_dict['inheritance_source_run_id']}]."
                        else:
                            pred_ts_str = f"on {historical_timestamp}" if historical_timestamp else "(ts unknown)"
                            inheritance_reason = f"Inherited label '{reason_label}' from predecessor [{testcase_data_dict['inheritance_source_run_id']}] {pred_ts_str}, origin details not found."
                        testcase_data_dict["inheritance_reason"] = inheritance_reason
                        logger.info(f"Set inheritance_reason for {current_test_name}: {inheritance_reason}") # MODIFIED

                        # Your existing log_similarity_score logic
                        testcase_data_dict["log_similarity_score"] = None
                        similarity_calculated = False
                        if self.embedding_model:
                            logger.info(f"Attempting to calculate log similarity for {current_test_name}") # MODIFIED
                            if current_log and origin_logs:
                                try:
                                    logger.info(f"Calculating similarity vs ORIGIN log for {current_test_name}") # MODIFIED
                                    embeddings = self.embedding_model.embed_documents([current_log, origin_logs])
                                    vec1 = np.array(embeddings[0]).reshape(1, -1)
                                    vec2 = np.array(embeddings[1]).reshape(1, -1)
                                    similarity = cosine_similarity(vec1, vec2)[0][0]
                                    testcase_data_dict["log_similarity_score"] = float(similarity)
                                    similarity_calculated = True
                                    logger.info(f"Log similarity score vs origin: {similarity:.4f} for {current_test_name}") # MODIFIED
                                except Exception as e: logger.error(f"Failed similarity vs origin for TC '{current_test_name}': {e}", exc_info=True)
                            elif current_log: 
                                predecessor_log = history.get("logs", "")
                                if predecessor_log:
                                    logger.info(f"Origin log missing, comparing vs predecessor for {current_test_name}") # MODIFIED
                                    try:
                                        embeddings = self.embedding_model.embed_documents([current_log, predecessor_log])
                                        vec1 = np.array(embeddings[0]).reshape(1, -1)
                                        vec2 = np.array(embeddings[1]).reshape(1, -1)
                                        similarity = cosine_similarity(vec1, vec2)[0][0]
                                        testcase_data_dict["log_similarity_score"] = float(similarity)
                                        similarity_calculated = True
                                        logger.info(f"Log similarity score vs predecessor (fallback): {similarity:.4f} for {current_test_name}") # MODIFIED
                                    except Exception as e: logger.error(f"Failed similarity vs predecessor for TC '{current_test_name}': {e}", exc_info=True)
                        if not similarity_calculated:
                            logger.info(f"Could not calculate log similarity for {current_test_name}") # MODIFIED
                            testcase_data_dict["log_similarity_score"] = "no logs provided for comparison"
                    except Exception as e_inherit:
                        logger.error(f"Error during {current_status} test inheritance for {current_test_name}: {e_inherit}", exc_info=True)
                        logger.info(f"FALLBACK to New Issue due to error for {current_test_name}") # MODIFIED
                        testcase_data_dict.update({"triage_status": "New", "inherited_label": False, "label": "", "bugs": []})
                elif not self.embedding_model and should_inherit:
                    logger.info(f"Cannot calculate log similarity for '{current_test_name}': Model unavailable.") # MODIFIED
                    testcase_data_dict["log_similarity_score"] = "model unavailable"
                else: 
                    logger.info(f"NEW ISSUE (no inheritance) for {current_status.upper()} TC {current_test_name}") # MODIFIED
                    testcase_data_dict.update({"triage_status": "New", "label": "", "bugs": []})
            else: 
                current_status = "passed"
                logger.info(f"Testcase {current_test_name} is PASSED") # MODIFIED
                testcase_data_dict.update({"status": current_status, "label": "Test Passed. No Label Required."})

            processed_tc_list.append(testcase_data_dict)
            logger.info(f"Finished processing TC: {current_test_name}, Status: {current_status}") # MODIFIED
        
        logger.info(f"Completed _create_testsuites with {len(processed_tc_list)} testcases processed") # MODIFIED
        return processed_tc_list