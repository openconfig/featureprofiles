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
            print("Initializing embedding model...")
            self.embeddings = Embeddings() # Instance of your Embeddings class
            self.embedding_model = self.embeddings.get_model() # Get the actual model
            print("Embedding model initialized successfully.")
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
                print("show_version file not found. Could not extract version/workspace.")

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
             print("Could not determine chain from run_info['submission_cmd']")

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
            print(f"Could not read show_version file {file_path}: {e}")
        except Exception as e: # Catch other potential errors during parsing
            print(f"Error parsing show_version file {file_path}: {e}")
        return version, workspace

    def _find_verification_origin(self, database, lineup, group, plan_id, testcase_name, start_timestamp):
        """
        Looks back through history for verification details. Prioritizes finding the
        most recent run with 'verified_by'. If not found, returns details of the
        oldest run checked during the search.
        """
        print(f"Origin search started: TC='{testcase_name}', Config={group}/{plan_id}/{lineup}, Before={start_timestamp}")
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
            if current_ts is None: print(f"Cannot backtrack: Missing timestamp."); break
            print(f"Lookup {i+1}/{MAX_VERIFICATION_LOOKUP_DEPTH}, searching before: {current_ts}")
            try:
                 historical_run = database.get_historical_testsuite(lineup, group, plan_id, before_timestamp=current_ts)
            except Exception as e: logger.error(f"DB error during lookup {i+1}: {e}"); break

            if not historical_run: print("No further historical run found."); break

            # Record this run as the oldest checked *so far* in this search
            # Do this *before* checking verified_by for this run
            oldest_run_id_checked = historical_run.get("run_id")
            oldest_timestamp_checked = historical_run.get("timestamp")
            print(f"Checking run {oldest_run_id_checked} @ {oldest_timestamp_checked}")

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
                    print(f"Found verification details for '{testcase_name}': Run=[{found_origin_run_id}], By=[{found_verified_by}]")
                    # *** Keep searching backwards *** to find the oldest run where the TC exists.
                    # The details stored above (found_*) hold the LATEST verification seen.
                else:
                    print(f"Run {oldest_run_id_checked} did not have 'verified_by' for TC '{testcase_name}'.")
            else:
                print(f"TC '{testcase_name}' not found in run {oldest_run_id_checked}. Stopping search for this TC.")
                break # Stop if the test case disappears

            # Check timestamp for next iteration
            if not next_ts or next_ts >= current_ts:
                 print(f"Timestamp issue in run {oldest_run_id_checked} (current='{current_ts}', next='{next_ts}'). Stopping.")
                 break
            current_ts = next_ts # Setup for the next loop

        if i == MAX_VERIFICATION_LOOKUP_DEPTH - 1:
            print(f"Origin search reached max depth ({MAX_VERIFICATION_LOOKUP_DEPTH}) for TC '{testcase_name}'.")


        if found_verified_by:
            # If we found a verification at some point, return those details
            print(f"Returning details from verification origin run: {found_origin_run_id}")
            return found_origin_run_id, found_origin_timestamp, found_verified_by, found_origin_logs
        elif oldest_run_id_checked:
             # If no verification was ever found, return details of the oldest run checked
             print(f"No 'verified_by' found. Returning details from oldest run checked: {oldest_run_id_checked}")
             # Logs are only relevant if verified_by was found, so return None for logs here
             return oldest_run_id_checked, oldest_timestamp_checked, None, None
        else:
             # If no historical runs were found at all in the loop
             print("No historical runs were successfully checked.")
             return None, None, None, None
         
         
    def _is_ai_generated_label(self, testcase):
        """
        Determine if a test case has an AI-generated label based on multiple indicators.
        This provides a centralized place to add new indicators in the future.
        """
        # Check for direct indicators from AI labeling
        if testcase.get("generated", False):
            return True
        
        if testcase.get("generated_label_details") is not None:
            return True
        
        if testcase.get("generated_score") is not None:
            return True
        
        if testcase.get("recalibration_candidate", False):
            return True
        
        # Check for "AI Suggested" pattern in triage_status or label
        label = testcase.get("label", "")
        if "ai suggested" in label.lower():
            return True
        
        # Check for the pattern where label exists but triage_status is still "New"
        # This often indicates an AI suggestion that hasn't been verified
        if label and testcase.get("triage_status") == "New":
            return True
        
        # Check for user feedback indicators
        if "user_verified_label" in testcase and not testcase.get("user_verified_label"):
            return True
        
        return False


    def get_testsuites(self, database, run_info):
        """Gather testsuite data to store into Database"""
        print(f"Starting get_testsuites for run_id: {run_info.get('firex_id', 'Unknown')}")
        documents = []
        current_run_id = run_info.get("firex_id", "Unknown")

        if self.root is None:
            logger.error("XML root is None, cannot process test suites.")
            return documents

        # Visit all testsuites within a run
        print(f"Processing testsuites from XML root")
        for testsuite in self.root.findall("./testsuite"):
            stats = testsuite.attrib
            properties = testsuite.find("properties")
            testcases = testsuite.findall("testcase")

            if properties is None:
                print(f"Skipping testsuite without properties element. Attributes: {stats}")
                continue

            failures_count = int(stats.get("failures", 0))
            errors_count = int(stats.get("errors", 0))
            test_passed = failures_count + errors_count == 0
            current_run_timestamp = str(stats.get("timestamp", "N/A"))
            print(f"Processing testsuite with timestamp: {current_run_timestamp}, failures: {failures_count}, errors: {errors_count}")

            # Initialize base data dictionary
            data = {
                "group": run_info.get("group", "Unknown"),
                "efr": run_info.get("efr", "Unknown"), # <-- CORRECTED: Use 'efr' from run_info directly
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
                "bugs": [] # Initialize bugs as empty
            }
            print(f"Initialized base data: group={data['group']}, lineup={data['lineup']}, run_id={data['run_id']}, efr={data['efr']}")

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
            print(f"Testsuite framework: {framework}")

            if properties is not None:
                if framework == "cafy2":
                    print("Processing as CAFY2 framework")
                    for p in properties.findall("property"):
                        prop_name = p.get("name")
                        if prop_name in cafy_keys_mappings:
                            data[cafy_keys_mappings[prop_name]] = p.get("value")
                            print(f"Set {cafy_keys_mappings[prop_name]}={p.get('value')}")
                else: # Assume B4 or other framework
                    print("Processing as B4 or other framework")
                    for p in properties.findall("property"):
                        prop_name = p.get("name")
                        if prop_name in b4_keys:
                            data[prop_name.replace("test.", "")] = p.get("value")
                            print(f"Set {prop_name.replace('test.', '')}={p.get('value')}")

            # Grab historical testsuite if it exists
            historial_testsuite = None
            historical_timestamp = None # Immediate predecessor details
            plan_id_for_lookup = data.get("plan_id")
            group_for_lookup = data.get("group")
            lineup_for_lookup = data.get("lineup")
            print(f"Historical lookup parameters: plan_id={plan_id_for_lookup}, group={group_for_lookup}, lineup={lineup_for_lookup}")

            # Special case for b4-featureprofiles - try with just group and lineup
            if group_for_lookup == "b4-featureprofiles" and lineup_for_lookup != "Unknown":
                try:
                    print(f"Attempting special lookup for b4-featureprofiles group with lineup: {lineup_for_lookup}")
                    if hasattr(database, 'get_historical_testsuite') and callable(getattr(database, 'get_historical_testsuite')):
                        # Try to find recent tests from the same group/lineup, even without plan_id
                        historial_testsuite = database.get_historical_testsuite(lineup_for_lookup, group_for_lookup, plan_id_for_lookup)
                        
                        if historial_testsuite:
                            retrieved_run_id = historial_testsuite.get('run_id', 'N/A')
                            retrieved_ts = historial_testsuite.get("timestamp", 'N/A')
                            print(f"SUCCESS: Retrieved predecessor for b4-featureprofiles: run_id=[{retrieved_run_id}], timestamp=[{retrieved_ts}]")
                            historical_timestamp = historial_testsuite.get("timestamp")
                        else:
                            print(f"No predecessor found for special b4-featureprofiles lookup with {group_for_lookup}/{lineup_for_lookup}")
                    else:
                        print("Database object does not have a callable 'get_historical_testsuite' method.")
                except Exception as e:
                    logger.error(f"Error in special lookup for b4-featureprofiles: {e}", exc_info=True)
            # Standard lookup requiring all three keys
            elif plan_id_for_lookup and group_for_lookup != "Unknown" and lineup_for_lookup != "Unknown":
                try:
                    print(f"Attempting standard historical lookup with plan_id={plan_id_for_lookup}")
                    if hasattr(database, 'get_historical_testsuite') and callable(getattr(database, 'get_historical_testsuite')):
                        historial_testsuite = database.get_historical_testsuite(lineup_for_lookup, group_for_lookup, plan_id_for_lookup) # No before_timestamp -> gets latest
                        if historial_testsuite:
                            retrieved_run_id = historial_testsuite.get('run_id', 'N/A')
                            retrieved_ts = historial_testsuite.get("timestamp", 'N/A')
                            print(f"SUCCESS: Retrieved immediate predecessor for inheritance: run_id=[{retrieved_run_id}], timestamp=[{retrieved_ts}]")
                            historical_timestamp = historial_testsuite.get("timestamp") # Timestamp of immediate predecessor
                        else:
                            print(f"No immediate predecessor found for {group_for_lookup}/{plan_id_for_lookup}/{lineup_for_lookup}.")
                    else:
                        print("Database object does not have a callable 'get_historical_testsuite' method.")
                except Exception as e:
                    logger.error(f"Error calling get_historical_testsuite for {group_for_lookup}/{plan_id_for_lookup}/{lineup_for_lookup}: {e}", exc_info=True)
            else:
                print(f"Skipping historical lookup due to missing keys: plan_id={plan_id_for_lookup}, group={group_for_lookup}, lineup={lineup_for_lookup}")

            # Create a dictionary to track which test cases had AI-generated labels in history
            ai_generated_testcases = {}
            
            # Examine historical data before processing test cases
            if historial_testsuite:
                print(f"Checking historical test cases for AI-generated indicators")
                ai_generated_count = 0
                for historical_tc in historial_testsuite.get("testcases", []):
                    tc_name = historical_tc.get("name", "")
                    
                    # Use helper method to check for AI-generated indicators
                    is_ai_generated = self._is_ai_generated_label(historical_tc)
                    
                    # Store this information for each test case
                    ai_generated_testcases[tc_name] = is_ai_generated
                    
                    if is_ai_generated:
                        ai_generated_count += 1
                        print(f"Found AI-generated test case in historical data: {tc_name}")
                
                if ai_generated_count > 0:
                    print(f"Found {ai_generated_count} AI-generated test cases in historical data")

            # Initialize data["bugs"] based on inheritance from predecessor
            # This block is what currently populates data["bugs"]
            if historial_testsuite:
                print(f"Processing bug inheritance from historical testsuite")
                for bug in historial_testsuite.get("bugs", []):
                    name = bug.get("name")
                    bug_type = bug.get("type")
                    if name and bug_type:
                        try:
                            print(f"Inheriting bug {name} of type {bug_type}")
                            # Make bug type check case-insensitive
                            if bug_type.lower() in ["ddts", "techzone", "github"]:
                                # Map to proper class name based on lowercase
                                class_mapping = {
                                    "ddts": "ddts",
                                    "techzone": "techzone",
                                    "github": "github"
                                }
                                # Get the appropriate module name for lookup
                                module_name = class_mapping.get(bug_type.lower())
                                if module_name:
                                    inherited_bug = globals()[module_name].inherit(name)
                                    # Only append the bug if it's not None (filtered out)
                                    if inherited_bug is not None:
                                        data["bugs"].append(inherited_bug)
                                        print(f"Successfully inherited bug {name}")
                                        if bug_type.lower() == "ddts" and test_passed and ddts.is_open(name):
                                            data["health"] = "unstable"
                                            print(f"Set health to 'unstable' due to open DDTS bug {name}")
                                else:
                                    print(f"Unknown bug type '{bug_type}' for bug '{name}'")
                        except Exception as e:
                            logger.error(f"Error inheriting bug {name} of type {bug_type}: {e}", exc_info=True)
                    else:
                        print(f"Skipping bug inheritance due to missing name or type: {bug}")

            # --- ADDED LOGIC FOR BUG CLEARING (IF TEST PASSES) ---
            # This is the logic that's missing for Day 5 to clear the bugs.
            # If the current testsuite passed and there are inherited bugs, clear them.
            if test_passed and data["bugs"]:
                print(f"Testsuite passed for run_id {current_run_id}. Clearing {len(data['bugs'])} inherited bugs.")
                data["bugs"] = [] # Clear the bugs array
                data["health"] = "ok" # Ensure health is 'ok' if test passed and bugs cleared
            # --- END ADDED LOGIC ---

            # Pass database object AND immediate predecessor details AND lookup keys to _create_testsuites
            print(f"Calling _create_testsuites with {len(testcases)} test cases")
            data["testcases"] = self._create_testsuites(
                database,
                testcases,
                historial_testsuite,
                historical_timestamp,
                group_for_lookup,
                plan_id_for_lookup,
                lineup_for_lookup,
                ai_generated_testcases  # Dictionary of AI-generated test cases
            )
            
            print(f"Processed {len(data['testcases'])} test cases")
            documents.append(data)
        
        print(f"Completed get_testsuites with {len(documents)} documents")
        return documents
    
    
    def _validate_log_path_for_testsuite(self, path, testsuite_name, testsuite_hash):
        """
        Validate that a log path belongs to a specific testsuite.
        
        Args:
            path: The log path to validate
            testsuite_name: The name/plan_id of the testsuite
            testsuite_hash: The hash of the testsuite
            
        Returns:
            bool: True if the path is valid for this testsuite, False otherwise
        """
        if not path:
            print("Cannot validate empty path")
            return False
            
        print(f"Validating path '{path}' for testsuite '{testsuite_name}' with hash '{testsuite_hash}'")
                
        # Direct validation by testsuite name
        if testsuite_name and testsuite_name in path:
            print(f"Path contains testsuite name '{testsuite_name}'")
            return True
        
        # Validate by testsuite hash
        if testsuite_hash and testsuite_hash in path:
            print(f"Path contains testsuite hash '{testsuite_hash}'")
            return True
        
        # Generate variants of the testsuite name
        name_variants = []
        if testsuite_name:
            name_variants = [
                testsuite_name,
                testsuite_name.replace(".", "_"),
                testsuite_name.replace("-", "_"),
                testsuite_name.replace("-", "_").replace(".", "_")
            ]
            
            # Get base name (e.g. "RT-5" from "RT-5.3")
            if "." in testsuite_name:
                base_name = testsuite_name.split(".")[0]
                name_variants.append(base_name)
                name_variants.append(base_name.replace("-", "_"))
                
            # Convert "gNMI-1.14" to "gNMI_1_BB495248" format
            if "-" in testsuite_name and "." in testsuite_name:
                parts = testsuite_name.split("-")
                if len(parts) > 1:
                    prefix = parts[0]
                    # Replace any dots with underscores in the version part
                    version_part = parts[1].replace(".", "_")
                    variant = f"{prefix}_{version_part}"
                    if variant not in name_variants:
                        name_variants.append(variant)
        
        # Remove duplicates
        name_variants = list(set(name_variants))
        print(f"Testing path against variants: {name_variants}")
        
        # Check if any of the variants match
        for variant in name_variants:
            if variant and variant in path:
                print(f"Path contains variant '{variant}'")
                return True
        
        # Special case for directories like "tests_logs/gNMI_1_BB495248" 
        # Check if the path format indicates a hash-based directory that might match our testsuite
        if "_" in path and testsuite_name:
            parts = path.split("/")
            for part in parts:
                if "_" in part:
                    part_segments = part.split("_")
                    # Check if the prefix matches the testsuite name prefix
                    if len(part_segments) >= 2:
                        prefix = part_segments[0]
                        # Check if this prefix matches the start of our testsuite name
                        if testsuite_name.startswith(prefix):
                            print(f"Path contains matching prefix '{prefix}' from testsuite name '{testsuite_name}'")
                            return True
                
        print(f"Path does not match any variant of testsuite '{testsuite_name}' or hash '{testsuite_hash}'")
        return False


    def _create_testsuites(self, database, testcases, historial_testsuite, historical_timestamp, group, plan_id, lineup, ai_generated_testcases=None):
        """Process test cases and handle inheritance, skipping AI-generated labels"""
        print(f"Starting _create_testsuites with {len(testcases)} testcases, plan_id={plan_id}")
        
        if ai_generated_testcases is None:
            ai_generated_testcases = {}
        
        # Create a mapping of test case names to their testsuite elements and properties
        testcase_mapping = {}
        
        # Find all testsuites in the XML and map each testcase to its parent testsuite
        all_testsuites = self.root.findall(".//testsuite")
        print(f"Found {len(all_testsuites)} testsuite elements in XML")
        
        # For dummy test cases or test cases without a specific testsuite, track the testsuite by plan_id
        plan_id_testsuite_map = {}
        
        for testsuite in all_testsuites:
            # Extract the testsuite's plan_id for validation
            ts_properties = testsuite.find("properties")
            if ts_properties is None:
                print(f"Skipping testsuite without properties element")
                continue
                
            # Get plan_id from various possible property names
            ts_plan_id = None
            for prop_name in ["test.plan_id", "testsuite_name", "plan_id"]:
                plan_id_prop = ts_properties.find(f"./property[@name='{prop_name}']")
                if plan_id_prop is not None:
                    ts_plan_id = plan_id_prop.get("value")
                    break
                    
            if ts_plan_id is None:
                print(f"Skipping testsuite without plan_id")
                continue
                
            # Get the testsuite hash for verification
            ts_hash = None
            hash_prop = ts_properties.find("./property[@name='testsuite_hash']")
            if hash_prop is not None:
                ts_hash = hash_prop.get("value")
                
            print(f"Processing testsuite with plan_id={ts_plan_id}, hash={ts_hash}")
            
            # Store all the properties for this testsuite to ensure we capture everything
            ts_props = {}
            for prop in ts_properties.findall("./property"):
                prop_name = prop.get("name")
                prop_value = prop.get("value")
                if prop_name and prop_value:
                    ts_props[prop_name] = prop_value
            
            # Store relevant details for easier access
            testsuite_details = {
                "plan_id": ts_plan_id,
                "testsuite_hash": ts_hash,
                "properties": ts_props,
                "properties_element": ts_properties
            }
            
            # Add to plan_id map for dummy test cases
            plan_id_testsuite_map[ts_plan_id] = testsuite_details
            
            # Map all test cases in this testsuite
            for tc in testsuite.findall("testcase"):
                tc_name = tc.get("name")
                if tc_name:
                    # Store a clean copy of the properties for this specific test case
                    testcase_mapping[tc_name] = dict(testsuite_details)
                    print(f"Mapped test case '{tc_name}' to testsuite with plan_id={ts_plan_id}")
        
        # Now process each test case with its proper context
        testsuites = []
        for testcase in testcases:
            current_test_name = testcase.get("name", "Unnamed Test")
            print(f"Processing testcase: {current_test_name}")
            
            # Get the mapped testsuite properties for this test case
            tc_props = testcase_mapping.get(current_test_name)
            
            # If no mapping found but we have a plan_id, use that mapping instead
            # This is crucial for dummy test cases or test cases without direct mapping
            if tc_props is None and plan_id in plan_id_testsuite_map:
                tc_props = plan_id_testsuite_map[plan_id]
                print(f"No direct mapping found for {current_test_name}, using plan_id={plan_id} mapping instead")
                
            # If still no mapping, use defaults
            if tc_props is None:
                print(f"No testsuite mapping found for test case {current_test_name}")
                # Continue processing even without mapping - just use default values
                tc_plan_id = plan_id
                tc_hash = None
                tc_properties = {}
                print(f"Using default plan_id={tc_plan_id} without specific testsuite mapping")
            else:
                # Extract the test-specific properties we need
                tc_plan_id = tc_props["plan_id"]
                tc_hash = tc_props["testsuite_hash"]
                tc_properties = tc_props["properties"]
                
                # IMPORTANT: For dummy test cases, ensure the plan_id is correct
                if current_test_name == "dummy" and tc_plan_id != plan_id:
                    print(f"WARNING: Mapped dummy test case to wrong plan_id: {tc_plan_id}, should be {plan_id}")
                    # Override with correct plan_id properties if available
                    if plan_id in plan_id_testsuite_map:
                        tc_props = plan_id_testsuite_map[plan_id]
                        tc_plan_id = tc_props["plan_id"]
                        tc_hash = tc_props["testsuite_hash"]
                        tc_properties = tc_props["properties"]
                        print(f"Corrected dummy test case mapping to plan_id={tc_plan_id}")
                
                print(f"Using properties for {current_test_name} with plan_id={tc_plan_id}, hash={tc_hash}")
                
                # VALIDATION: Double check the test case has properties from the right testsuite
                if tc_plan_id != plan_id:
                    print(f"WARNING: Test case {current_test_name} is using properties from plan_id={tc_plan_id}, but expected {plan_id}")
                    # Try to correct if possible
                    if plan_id in plan_id_testsuite_map:
                        correct_props = plan_id_testsuite_map[plan_id]
                        tc_plan_id = correct_props["plan_id"]
                        tc_hash = correct_props["testsuite_hash"]
                        tc_properties = correct_props["properties"]
                        print(f"Corrected test case {current_test_name} to use properties from plan_id={tc_plan_id}")
            
            # Clean extraction of specific properties for this test case
            root_path = tc_properties.get("testsuite_root")
            test_log_dir = tc_properties.get("test_log_directory")
            log_path = tc_properties.get("log")
            
            print(f"For {current_test_name}: root_path={root_path}, test_log_dir={test_log_dir}, log_path={log_path}")
            
            # Check if this test was AI-generated in historical data
            is_ai_generated = ai_generated_testcases.get(current_test_name, False)
            
            if is_ai_generated:
                print(f"SKIPPING INHERITANCE for AI-generated test case: {current_test_name}")
            
            # Only allow inheritance if not AI-generated
            inheritance_possible = historial_testsuite is not None and not is_ai_generated
            history = None  # Immediate predecessor's TC data
            
            if inheritance_possible:
                print(f"Checking for historical data for testcase: {current_test_name}")
                
                # Extract historical testsuite info for validation
                historical_plan_id = historial_testsuite.get("plan_id")
                historical_hash = historial_testsuite.get("testsuite_hash")
                
                print(f"Historical testsuite: plan_id={historical_plan_id}, hash={historical_hash}")
                print(f"Current testsuite: plan_id={tc_plan_id}, hash={tc_hash}")
                
                # Only inherit if test suites match
                if tc_plan_id == historical_plan_id or tc_hash == historical_hash:
                    print(f"Testsuites match, looking for matching test case")
                    for e in historial_testsuite.get("testcases", []):
                        if e.get("name") == current_test_name: 
                            # Double-check for AI-generated indicators at this level too
                            if self._is_ai_generated_label(e):
                                print(f"Found AI-generated indicators in historical test case during match lookup: {current_test_name}")
                                history = None
                                break
                            
                            print(f"Found historical match for testcase: {current_test_name}")
                            history = e 
                            break
                else:
                    print(f"WARNING: Testsuites don't match - not inheriting across testsuite boundaries!")
                    print(f"   Current: {tc_plan_id} != Historical: {historical_plan_id}")
                    print(f"   Current hash: {tc_hash} != Historical hash: {historical_hash}")
                
                if history is None:
                    print(f"No applicable history found for testcase: {current_test_name}")
                else:
                    print(f"Historical data status: {history.get('status')}, label: {history.get('label', 'None')}")
            
            # Initialize basic fields common to all statuses
            testcase_data = {"name": current_test_name, "time": float(testcase.get("time", 0))}
            print(f"Initialized basic testcase data for {current_test_name}")

            failure_el = testcase.find("failure")
            error_el = testcase.find("error")
            skipped_el = testcase.find("skipped")
            current_status = "passed"
            current_log = None

            # Determine Status and Handle Status-Specific Fields
            if skipped_el is not None:
                current_status = "skipped"
                print(f"Testcase {current_test_name} is SKIPPED")
                testcase_data["status"] = current_status
                testcase_data["label"] = "Test Skipped. No Label Required."
                # No other fields needed for skipped

            elif error_el is not None:  # Any error element, with or without message
                
                # First check if it matches our existing aborted condition
                if error_el.get("message") is None:
                    current_status = "aborted"
                    print(f"Testcase {current_test_name} is ABORTED (empty error)")
                else:
                    current_status = "failed"
                    print(f"Testcase {current_test_name} is FAILED (error with message)")
                
                testcase_data["status"] = current_status
                
                # Extract logs regardless of status
                # Try direct log extraction from the error element first
                text = error_el.text
                current_log = str(text).strip() if text else ""
                
                # If no logs from error element, try file-based extraction with the test case's specific properties
                if not current_log and root_path and (test_log_dir or log_path):
                    try:
                        print(f"Attempting to extract logs for {current_test_name} using its specific properties")
                        
                        # First try using test_log_directory
                        if test_log_dir:
                            # Validate that the log directory belongs to the current test suite
                            if self._validate_log_path_for_testsuite(test_log_dir, tc_plan_id, tc_hash):
                                print(f"Validated log directory belongs to test suite {tc_plan_id}")
                                full_log_path = os.path.join(root_path, test_log_dir, "script_console_output.txt")
                                print(f"Using validated log path: {full_log_path}")
                            
                                if os.path.exists(full_log_path):
                                    with os.popen(f"tail -n 50 {full_log_path}") as pipe:
                                        tail_output = pipe.read()
                                        if tail_output:
                                            current_log = tail_output
                                            print(f"Successfully extracted {len(current_log)} chars from test_log_dir")
                                        else:
                                            print(f"Log file exists but tail command returned no output: {full_log_path}")
                                else:
                                    print(f"Log file not found: {full_log_path}")
                            else:
                                print(f"Log directory {test_log_dir} does not match test suite {tc_plan_id}")
                        
                        # If still no log, try log property as fallback
                        if not current_log and log_path:
                            # Validate that the log path belongs to the current test suite
                            if self._validate_log_path_for_testsuite(log_path, tc_plan_id, tc_hash):
                                print(f"Validated log path belongs to test suite {tc_plan_id}")
                                full_log_path = os.path.join(root_path, log_path)
                                print(f"Using validated log_path: {full_log_path}")
                                
                                if os.path.exists(full_log_path):
                                    with os.popen(f"tail -n 100 {full_log_path}") as pipe:
                                        tail_output = pipe.read()
                                        if tail_output:
                                            current_log = tail_output
                                            print(f"Successfully extracted {len(current_log)} chars from log_path")
                                        else:
                                            print(f"Log file exists but tail command returned no output: {full_log_path}")
                                else:
                                    print(f"Log file not found: {full_log_path}")
                            else:
                                print(f"Log path {log_path} does not match test suite {tc_plan_id}")
                                
                        # If still no logs, try constructing a path using the hash and plan ID
                        if not current_log and tc_hash:
                            constructed_path = f"tests_logs/{tc_hash}/script_console_output.txt"
                            print(f"Trying constructed path based on hash: {constructed_path}")
                            
                            # Validate the constructed path
                            if self._validate_log_path_for_testsuite(constructed_path, tc_plan_id, tc_hash):
                                full_log_path = os.path.join(root_path, constructed_path)
                                
                                if os.path.exists(full_log_path):
                                    with os.popen(f"tail -n 100 {full_log_path}") as pipe:
                                        tail_output = pipe.read()
                                        if tail_output:
                                            current_log = tail_output
                                            print(f"Successfully extracted {len(current_log)} chars from constructed hash path")
                                        else:
                                            print(f"Log file exists but tail command returned no output: {full_log_path}")
                                else:
                                    print(f"Constructed log file not found: {full_log_path}")
                            else:
                                print(f"Constructed log path {constructed_path} does not match test suite {tc_plan_id}")
                                
                        # Last resort: try to find a log file by searching the testsuite root for matching files
                        if not current_log and root_path and tc_plan_id and tc_hash:
                            print(f"Attempting to search for log files matching test suite {tc_plan_id}")
                            try:
                                # Search for log files that might match our testsuite
                                search_command = f"find {root_path}/tests_logs -name 'script_console_output.txt' | grep -i '{tc_plan_id}\\|{tc_hash}' | head -1"
                                with os.popen(search_command) as pipe:
                                    matching_file = pipe.read().strip()
                                    if matching_file and os.path.exists(matching_file):
                                        print(f"Found potential log file via search: {matching_file}")
                                        with os.popen(f"tail -n 100 {matching_file}") as log_pipe:
                                            tail_output = log_pipe.read()
                                            if tail_output:
                                                current_log = tail_output
                                                print(f"Successfully extracted {len(current_log)} chars from found log file")
                                            else:
                                                print(f"Found log file exists but returned no output: {matching_file}")
                                    else:
                                        print(f"No matching log files found via search")
                            except Exception as search_e:
                                print(f"Error searching for log files: {search_e}")
                    except Exception as e:
                        logger.error(f"Error extracting logs for {current_test_name}: {e}", exc_info=True)
                
                # Set message and logs
                testcase_data["message"] = "Aborted" if current_status == "aborted" else "Failed"
                testcase_data["logs"] = current_log
                print(f"Set {len(current_log) if current_log else 0} chars of logs for {current_test_name}")

                # Always set inherited_label to False by default
                testcase_data["inherited_label"] = False
                
                # Check if we should inherit from history - MODIFIED to prioritize label_id
                should_inherit = False
                if history:
                    print(f"Checking inheritance criteria for {current_status.upper()} testcase {current_test_name}")
                    # First try to match by label_id if it exists in history
                    if history.get("label_id") and history.get("label", "").strip():
                        should_inherit = True
                        print(f"Will inherit label for '{current_test_name}' based on label_id match")
                    # Fall back to the old status-based logic
                    elif history.get("status") == current_status and history.get("label", "").strip():
                        should_inherit = True
                        print(f"Will inherit label for '{current_test_name}' based on status match (fallback)")
                    else:
                        print(f"No inheritance criteria met for {current_status.upper()} testcase {current_test_name}")
                    
                if should_inherit:  # Inherit for ABORTED or FAILED
                    print(f"INHERITING LABEL for {current_status.upper()} testcase {current_test_name}")
                    try:
                        # Populate ALL relevant fields ONLY if inheriting
                        testcase_data["inherited_label"] = True
                        # Set triage_status to "Resolved" if there's a meaningful label
                        testcase_data["triage_status"] = "Resolved"
                        testcase_data["label"] = history.get("label", "")
                        testcase_data["bugs"] = history.get("bugs", [])
                        testcase_data["label_id"] = history.get("label_id", "")
                        print(f"Set inherited label: '{testcase_data['label']}' for {current_test_name}")

                        # Find verification origin
                        print(f"Looking up verification origin for {current_test_name}")
                        origin_run_id, origin_timestamp, origin_verified_by, origin_logs = self._find_verification_origin(
                            database, lineup, group, tc_plan_id, current_test_name, historical_timestamp
                        )
                        print(f"Found verification origin: run_id={origin_run_id}, verified_by={origin_verified_by}")
                        
                        testcase_data["original_verification_run_id"] = origin_run_id
                        testcase_data["verified_by"] = origin_verified_by if origin_verified_by is not None else "Unknown"
                        testcase_data["inheritance_date"] = origin_timestamp

                        predecessor_run_id = historial_testsuite.get("run_id", "Unknown") if historial_testsuite else "Unknown"
                        testcase_data["inheritance_source_run_id"] = predecessor_run_id

                        # Update Reason String
                        reason_label = history.get('label', '')
                        reason_ts_str = f"on {origin_timestamp}" if origin_timestamp else "(ts unknown)"
                        reason_verified_by = testcase_data["verified_by"]
                        
                        if origin_run_id and reason_verified_by != "Unknown":
                            inheritance_reason = f"Inherited label '{reason_label}' (verified by {reason_verified_by} in run [{origin_run_id}] {reason_ts_str}) via predecessor [{predecessor_run_id}]."
                        elif origin_run_id:
                            inheritance_reason = f"Inherited label '{reason_label}' (origin run [{origin_run_id}] {reason_ts_str}, verified_by Unknown) via predecessor [{predecessor_run_id}]."
                        else:
                            pred_ts_str = f"on {historical_timestamp}" if historical_timestamp else "(ts unknown)"
                            inheritance_reason = f"Inherited label '{reason_label}' from predecessor [{predecessor_run_id}] {pred_ts_str}, origin details not found."
                        
                        testcase_data["inheritance_reason"] = inheritance_reason
                        print(f"Set inheritance_reason for {current_test_name}: {inheritance_reason}")

                        # Calculate Log Similarity (vs ORIGIN log if available, else predecessor)
                        # Initialize score to None ONLY when inheriting
                        testcase_data["log_similarity_score"] = None
                        similarity_calculated = False
                        
                        if self.embedding_model:
                            print(f"Attempting to calculate log similarity for {current_test_name}")
                            if current_log and origin_logs:
                                try:
                                    print(f"Calculating similarity vs ORIGIN log for {current_test_name}")
                                    embeddings = self.embedding_model.embed_documents([current_log, origin_logs])
                                    vec1 = np.array(embeddings[0]).reshape(1, -1)
                                    vec2 = np.array(embeddings[1]).reshape(1, -1)
                                    similarity = cosine_similarity(vec1, vec2)[0][0]
                                    testcase_data["log_similarity_score"] = float(similarity)
                                    similarity_calculated = True
                                    print(f"Log similarity score vs origin: {similarity:.4f} for {current_test_name}")
                                except Exception as e:
                                    logger.error(f"Failed similarity vs origin for TC '{current_test_name}': {e}", exc_info=True)
                            elif current_log:  # Fallback: Compare to predecessor if origin logs missing
                                predecessor_log = history.get("logs", "")
                                if predecessor_log:
                                    print(f"Origin log missing, comparing vs predecessor for {current_test_name}")
                                    try:
                                        embeddings = self.embedding_model.embed_documents([current_log, predecessor_log])
                                        vec1 = np.array(embeddings[0]).reshape(1, -1)
                                        vec2 = np.array(embeddings[1]).reshape(1, -1)
                                        similarity = cosine_similarity(vec1, vec2)[0][0]
                                        testcase_data["log_similarity_score"] = float(similarity)
                                        similarity_calculated = True
                                        print(f"Log similarity score vs predecessor (fallback): {similarity:.4f} for {current_test_name}")
                                    except Exception as e:
                                        logger.error(f"Failed similarity vs predecessor for TC '{current_test_name}': {e}", exc_info=True)

                        # If similarity calculation was not successful for any reason, set to the string
                        if not similarity_calculated:
                            print(f"Could not calculate log similarity for {current_test_name}")
                            testcase_data["log_similarity_score"] = "no logs provided for comparison"
                    except Exception as e:
                        logger.error(f"Error during {current_status} test inheritance for {current_test_name}: {e}", exc_info=True)
                        # Fallback if verification origin fails
                        print(f"FALLBACK to New Issue due to error for {current_test_name}")
                        testcase_data["triage_status"] = "New"
                        testcase_data["inherited_label"] = False
                elif not self.embedding_model and should_inherit:
                    print(f"Cannot calculate log similarity for '{current_test_name}': Model unavailable.")
                    testcase_data["log_similarity_score"] = "model unavailable"
                else:  # Failed or Aborted, but no inheritance (New Issue)
                    print(f"NEW ISSUE (no inheritance) for {current_status.upper()} testcase {current_test_name}")
                    testcase_data["triage_status"] = "New"
                    testcase_data["label"] = ""  # Needs labeling
                    testcase_data["bugs"] = []
                    # No additional fields needed for new issues

            elif (error_el is not None and error_el.get("message")) or failure_el is not None:
                current_status = "failed"
                print(f"Testcase {current_test_name} is FAILED")
                testcase_data["status"] = current_status
                text = error_el.text if error_el is not None else failure_el.text
                current_log = str(text).strip() if text else ""
                testcase_data["message"] = "Failed"
                testcase_data["logs"] = current_log
                print(f"Set logs for {current_test_name} (length: {len(current_log) if current_log else 0})")
                
                # Always set inherited_label to False by default
                testcase_data["inherited_label"] = False
                
                # Check if we should inherit from history - MODIFIED to prioritize label_id
                should_inherit = False
                if history:
                    print(f"Checking inheritance criteria for FAILED testcase {current_test_name}")
                    # First try to match by label_id if it exists in history
                    if history.get("label_id") and history.get("label", "").strip():
                        should_inherit = True
                        print(f"Will inherit label for '{current_test_name}' based on label_id match")
                    # Fall back to the old status-based logic
                    elif history.get("status") == "failed" and history.get("label", "").strip():
                        should_inherit = True
                        print(f"Will inherit label for '{current_test_name}' based on status match (fallback)")
                    else:
                        print(f"No inheritance criteria met for FAILED testcase {current_test_name}")
                    
                if should_inherit:  # Inherit for FAILED
                    print(f"INHERITING LABEL for FAILED testcase {current_test_name}")
                    try:
                        # Populate ALL relevant fields ONLY if inheriting
                        testcase_data["inherited_label"] = True
                        # Set triage_status to "Resolved" if there's a meaningful label
                        testcase_data["triage_status"] = "Resolved"
                        testcase_data["label"] = history.get("label", "")
                        testcase_data["bugs"] = history.get("bugs", [])
                        testcase_data["label_id"] = history.get("label_id", "")
                        print(f"Set inherited label: '{testcase_data['label']}' for {current_test_name}")

                        # Find verification origin
                        print(f"Looking up verification origin for {current_test_name}")
                        origin_run_id, origin_timestamp, origin_verified_by, origin_logs = self._find_verification_origin(
                            database, lineup, group, tc_plan_id, current_test_name, historical_timestamp
                        )
                        print(f"Found verification origin: run_id={origin_run_id}, verified_by={origin_verified_by}")
                        
                        testcase_data["original_verification_run_id"] = origin_run_id
                        testcase_data["verified_by"] = origin_verified_by if origin_verified_by is not None else "Unknown"
                        testcase_data["inheritance_date"] = origin_timestamp

                        predecessor_run_id = historial_testsuite.get("run_id", "Unknown") if historial_testsuite else "Unknown"
                        testcase_data["inheritance_source_run_id"] = predecessor_run_id

                        # Update Reason String
                        reason_label = history.get('label', '')
                        reason_ts_str = f"on {origin_timestamp}" if origin_timestamp else "(ts unknown)"
                        reason_verified_by = testcase_data["verified_by"]
                        
                        if origin_run_id and reason_verified_by != "Unknown":
                            inheritance_reason = f"Inherited label '{reason_label}' (verified by {reason_verified_by} in run [{origin_run_id}] {reason_ts_str}) via predecessor [{predecessor_run_id}]."
                        elif origin_run_id:
                            inheritance_reason = f"Inherited label '{reason_label}' (origin run [{origin_run_id}] {reason_ts_str}, verified_by Unknown) via predecessor [{predecessor_run_id}]."
                        else:
                            pred_ts_str = f"on {historical_timestamp}" if historical_timestamp else "(ts unknown)"
                            inheritance_reason = f"Inherited label '{reason_label}' from predecessor [{predecessor_run_id}] {pred_ts_str}, origin details not found."
                        
                        testcase_data["inheritance_reason"] = inheritance_reason
                        print(f"Set inheritance_reason for {current_test_name}: {inheritance_reason}")

                        # Calculate Log Similarity (vs ORIGIN log if available, else predecessor)
                        # Initialize score to None ONLY when inheriting
                        testcase_data["log_similarity_score"] = None
                        similarity_calculated = False
                        
                        if self.embedding_model:
                            print(f"Attempting to calculate log similarity for {current_test_name}")
                            if current_log and origin_logs:
                                try:
                                    print(f"Calculating similarity vs ORIGIN log for {current_test_name}")
                                    embeddings = self.embedding_model.embed_documents([current_log, origin_logs])
                                    vec1 = np.array(embeddings[0]).reshape(1, -1)
                                    vec2 = np.array(embeddings[1]).reshape(1, -1)
                                    similarity = cosine_similarity(vec1, vec2)[0][0]
                                    testcase_data["log_similarity_score"] = float(similarity)
                                    similarity_calculated = True
                                    print(f"Log similarity score vs origin: {similarity:.4f} for {current_test_name}")
                                except Exception as e:
                                    logger.error(f"Failed similarity vs origin for TC '{current_test_name}': {e}", exc_info=True)
                            elif current_log:  # Fallback: Compare to predecessor if origin logs missing
                                predecessor_log = history.get("logs", "")
                                if predecessor_log:
                                    print(f"Origin log missing, comparing vs predecessor for {current_test_name}")
                                    try:
                                        embeddings = self.embedding_model.embed_documents([current_log, predecessor_log])
                                        vec1 = np.array(embeddings[0]).reshape(1, -1)
                                        vec2 = np.array(embeddings[1]).reshape(1, -1)
                                        similarity = cosine_similarity(vec1, vec2)[0][0]
                                        testcase_data["log_similarity_score"] = float(similarity)
                                        similarity_calculated = True
                                        print(f"Log similarity score vs predecessor (fallback): {similarity:.4f} for {current_test_name}")
                                    except Exception as e:
                                        logger.error(f"Failed similarity vs predecessor for TC '{current_test_name}': {e}", exc_info=True)

                        # If similarity calculation was not successful for any reason, set to the string
                        if not similarity_calculated:
                            print(f"Could not calculate log similarity for {current_test_name}")
                            testcase_data["log_similarity_score"] = "no logs provided for comparison"
                    except Exception as e:
                        logger.error(f"Error during failed test inheritance for {current_test_name}: {e}", exc_info=True)
                        # Fallback if verification origin fails
                        print(f"FALLBACK to New Issue due to error for {current_test_name}")
                        testcase_data["triage_status"] = "New"
                        testcase_data["inherited_label"] = False
                elif not self.embedding_model and should_inherit:
                    print(f"Cannot calculate log similarity for '{current_test_name}': Model unavailable.")
                    testcase_data["log_similarity_score"] = "model unavailable"
                else:  # Failed, but no inheritance (New Issue)
                    print(f"NEW ISSUE (no inheritance) for FAILED testcase {current_test_name}")
                    testcase_data["triage_status"] = "New"
                    testcase_data["label"] = ""  # Needs labeling
                    testcase_data["bugs"] = []
                    # No additional fields needed for new issues

            else:  # Passed
                current_status = "passed"
                print(f"Testcase {current_test_name} is PASSED")
                testcase_data["status"] = current_status
                testcase_data["label"] = "Test Passed. No Label Required."
                # No other fields needed for passed

            testsuites.append(testcase_data)
            print(f"Finished processing testcase: {current_test_name} with status: {current_status}")
        
        print(f"Completed _create_testsuites with {len(testsuites)} testcases processed")
        return testsuites