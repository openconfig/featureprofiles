# failed_code_collector.py
import re
import os
from collections import defaultdict
import logging

class FailedTestCaseCodeCollector:
    """
    Processes log snippets from failed test cases to identify specific failure
    locations (file:line). It then filters these locations to reduce redundancy
    from closely grouped errors in the same file and extracts contextual log lines
    around each significant failure point.

    Key functionalities include:
    - Regex-based extraction of file and line numbers from log text.
    - Proximity-based filtering of error locations within the same file.
    - Extraction of a configurable window of log lines (context) around each
      filtered error location.
    """
    _log_location_regex = re.compile(r'(?:[/\w.-]+/)*([\w.-]+\.(?:py|go)):(\d+):')

    def __init__(self, nearby_line_buffer=5, log_context_lines=10, logger_instance=None):
        """
        Initializes the FailedTestCaseCodeCollector.

        Args:
            nearby_line_buffer (int, optional):
                The +/- range to consider lines as 'nearby' for filtering.
                If two errors in the same file are within this many lines of each
                other, only the first one in a cluster is kept. Defaults to 5.
            log_context_lines (int, optional):
                The number of lines *before and after* a target failure line
                to include in the extracted code context. Defaults to 10.
            logger_instance (logging.Logger, optional):
                An optional logger instance for internal logging by this class.
                If None, a new logger is created for this module.
        """
        self.nearby_line_buffer = nearby_line_buffer
        self.log_context_lines = log_context_lines
        self.logger = logger_instance if logger_instance else logging.getLogger(__name__)
        self.logger.info(f"FailedTestCaseCodeCollector initialized with buffer={self.nearby_line_buffer}, context_lines={self.log_context_lines}")
        self._script_path_cache = {}  # Cache to avoid repeated file searches

    @staticmethod
    def sort_key_failed_code_path(item):
        """
        Provides a sort key for a list of dictionaries, where each dictionary
        represents a failed code path and is expected to have a 'file_name'
        key with a string value like 'script.py:123'.

        Sorts primarily by filename (alphabetically) and secondarily by
        line number (numerically).

        Args:
            item (dict): A dictionary expected to have a 'file_name' key.

        Returns:
            tuple: A tuple (filename_str, linenumber_int) suitable for sorting.
                   Returns a fallback tuple (filename_str, 0) if parsing fails.
        """
        try:
            parts = item['file_name'].split(':')
            filename = parts[0]
            linenum = int(parts[1])
            return (filename, linenum)
        except (IndexError, ValueError):
            # Fallback for unexpected format, ensures sorting doesn't break
            return (item.get('file_name', ''), 0)

    def extract_raw_locations(self, log_text):
        """
        Scans a given log text snippet to find all occurrences of file:line
        patterns matching Python or Go stack traces or log messages, based on
        the class's `_log_location_regex`.

        Args:
            log_text (str): The string containing the log data to be parsed.

        Returns:
            list[tuple[str, int]]:
                A list of unique tuples, where each tuple is
                (filename_str, linenumber_int). Filenames are basenames
                (e.g., 'script.py'). Returns an empty list if no matches are
                found or if log_text is empty or None.
        """
        if not log_text:
            return []
        found_locations = set()
        lines = log_text.splitlines()
        for line in lines:
            match = self._log_location_regex.search(line.strip())
            if match:
                filename = os.path.basename(match.group(1))
                line_number_str = match.group(2)
                try:
                    found_locations.add((filename, int(line_number_str)))
                except ValueError:
                    self.logger.warning(f"Could not convert line number '{line_number_str}' to int for file '{filename}' in log: {line[:100]}...")
        return list(found_locations)
    
    def find_script_full_path(self, filename):
        """
        Searches for the full path of a test script file within the feature directory.
        
        Args:
            filename (str): The basename of the file to search for (e.g., 'encap_decap_scale_test.go')
            
        Returns:
            str: The relative path from the repo root (e.g., 'feature/gribi/otg_tests/encap_decap_scale/encap_decap_scale_test.go')
                 or empty string if not found.
        """
        if not filename:
            return ""
            
        # Check cache first
        if filename in self._script_path_cache:
            return self._script_path_cache[filename]
            
        import subprocess
        
        try:
            # Search up to 5 levels up for the feature directory
            for i in range(1, 6):
                search_path = "../" * i + "feature"
                if os.path.exists(search_path):
                    cmd = ["find", search_path, "-type", "f", "-name", filename]
                    result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
                    
                    if result.stdout.strip():
                        # Found the file
                        full_path = result.stdout.strip().split('\n')[0]  # Take first match
                        # Convert to relative path from repo root
                        if "/feature/" in full_path:
                            relative_path = "feature/" + full_path.split("/feature/")[1]
                            self._script_path_cache[filename] = relative_path
                            self.logger.info(f"Found script full path for {filename}: {relative_path}")
                            return relative_path
                            
            self.logger.warning(f"Could not find full path for script: {filename}")
            self._script_path_cache[filename] = ""  # Cache negative result
            return ""
            
        except Exception as e:
            self.logger.error(f"Error searching for script file {filename}: {e}")
            return ""

    def _filter_nearby_locations_internal(self, locations_with_logs):
        """
        Filters a list of identified failure locations to remove entries that
        are very close to each other within the *same file*. "Closeness" is
        defined by `self.nearby_line_buffer`.

        It groups locations by filename, sorts them by line number, and then
        iterates through them. If a location is within `self.nearby_line_buffer`
        lines of a previously kept location in the same file, it's discarded.
        The log snippet associated with the *first* kept location in such a
        cluster is retained.

        Args:
            locations_with_logs (list[tuple[str, int, str]]):
                A list of tuples, where each tuple is
                (filename_str, linenumber_int, original_log_snippet_str).
                The `original_log_snippet_str` is the log block from which the
                filename and linenumber were extracted.

        Returns:
            list[tuple[str, int, str]]:
                A filtered list of tuples, preserving the
                (filename, linenumber, original_log_snippet) structure for
                the entries that are kept.
        """
        if not locations_with_logs:
            return []

        grouped_by_filename = defaultdict(list)
        for filename, linenum, log_snippet in locations_with_logs:
            grouped_by_filename[filename].append((linenum, log_snippet))

        final_filtered_tuples = []
        for filename, linenum_snippet_pairs in grouped_by_filename.items():
            if not linenum_snippet_pairs:
                continue
            
            # Sort pairs by line number for the current file
            sorted_pairs = sorted(linenum_snippet_pairs, key=lambda x: x[0])
            
            # Initialize last_kept_line to ensure the first entry in sorted_pairs is always kept
            last_kept_line = - (self.nearby_line_buffer + 100) # Well outside any possible initial range
            
            for current_line, current_snippet in sorted_pairs:
                if abs(current_line - last_kept_line) > self.nearby_line_buffer:
                    final_filtered_tuples.append((filename, current_line, current_snippet))
                    last_kept_line = current_line
        
        self.logger.debug(f"Filtered {len(locations_with_logs)} locations down to {len(final_filtered_tuples)}.")
        return final_filtered_tuples

    def _extract_log_context_internal(self, log_snippet, target_filename, target_linenum):
        """
        Extracts a contextual snippet of log lines (target line +/-
        `self.log_context_lines`) from a larger log snippet. It centers the
        context around the specified `target_filename` and `target_linenum`
        by re-scanning the provided `log_snippet` to pinpoint the exact line.

        Args:
            log_snippet (str):
                The block of log text (typically from a single failed test case)
                from which to extract context.
            target_filename (str):
                The basename of the file (e.g., 'script.py') to find within the snippet.
            target_linenum (int):
                The line number within `target_filename` to center the context around.

        Returns:
            str:
                A multi-line string containing the extracted log context.
                Returns a specific message or a portion of the snippet if the
                target line cannot be precisely re-located within the snippet.
        """
        if not log_snippet:
            return "(Log snippet was empty)"

        lines = log_snippet.splitlines()
        target_line_index_in_snippet = -1

        # Attempt to find the index of the target line within the provided snippet
        for i, line_text in enumerate(lines):
            match = self._log_location_regex.search(line_text.strip())
            if match:
                fname_in_line = os.path.basename(match.group(1))
                try:
                    lnum_in_line = int(match.group(2))
                    if fname_in_line == target_filename and lnum_in_line == target_linenum:
                        target_line_index_in_snippet = i
                        break # Found the primary line of interest
                except ValueError:
                    continue # Skip lines where line number isn't an int
        
        if target_line_index_in_snippet == -1:
            self.logger.warning(f"Target line {target_filename}:{target_linenum} not re-found in its own log snippet. "
                                f"Context extraction might be less accurate.")
            # Fallback: if target is not found, maybe return last N lines as a simple heuristic
            # or return a message indicating the issue.
            # For now, let's return a portion if the snippet is large, or the whole snippet if small.
            if len(lines) > (2 * self.log_context_lines + 1):
                 # Attempt to grab the end of the log, which is often where errors are
                context_lines_fallback = lines[-(2 * self.log_context_lines + 1):]
                return "\n".join(context_lines_fallback) + \
                       f"\n(Context Warning: Target {target_filename}:{target_linenum} not pinpointed in snippet)"
            return log_snippet # Return the whole snippet if it's small

        # Calculate start and end indices for context extraction
        start_index = max(0, target_line_index_in_snippet - self.log_context_lines)
        end_index = min(len(lines), target_line_index_in_snippet + self.log_context_lines + 1) # +1 because slice excludes end
        
        context_lines = lines[start_index:end_index]
        return "\n".join(context_lines)

    def _extract_code_from_source_file(self, full_path, line_number, context_lines=10):
        """
        Extracts actual source code from the file around the specified line number.
        
        Args:
            full_path (str): Full path to the source file
            line_number (int): The line number to center the context around
            context_lines (int): Number of lines before and after to include
            
        Returns:
            str: The extracted source code or an error message
        """
        try:
            # Try to find the file using the full path
            search_paths = []
            
            # If it's already a full path from repo root
            if full_path.startswith("feature/"):
                for i in range(1, 6):
                    search_paths.append("../" * i + full_path)
            
            # Also try to find it if we only have the filename
            filename = os.path.basename(full_path)
            if filename != full_path:
                # Already have full path, use it
                found_path = None
                for path in search_paths:
                    if os.path.exists(path):
                        found_path = path
                        break
            else:
                # Need to find the full path first
                found_full_path = self.find_script_full_path(filename)
                if found_full_path:
                    for i in range(1, 6):
                        path = "../" * i + found_full_path
                        if os.path.exists(path):
                            found_path = path
                            break
                else:
                    found_path = None
            
            if not found_path:
                return f"(Source file not found: {full_path})"
                
            # Read the file and extract context
            with open(found_path, 'r') as f:
                lines = f.readlines()
                
            # Calculate line range (1-indexed in display, 0-indexed in list)
            start_line = max(1, line_number - context_lines)
            end_line = min(len(lines), line_number + context_lines)
            
            # Extract lines with line numbers
            result_lines = []
            for i in range(start_line - 1, end_line):
                line_num = i + 1
                prefix = ">>> " if line_num == line_number else "    "
                result_lines.append(f"{prefix}{line_num:4d}: {lines[i].rstrip()}")
                
            return "\n".join(result_lines)
            
        except Exception as e:
            self.logger.error(f"Error extracting code from {full_path}:{line_number} - {e}")
            return f"(Error reading source: {str(e)})"
    
    def process_testsuite_failures(self, all_raw_failure_data_for_testsuite):
        """
        The main public method to orchestrate the full processing of raw failure
        data collected from all test cases within a single testsuite run.

        It performs the following steps:
        1. Filters the raw failure locations (which include their original log snippets)
           using `_filter_nearby_locations_internal` to consolidate nearby errors
           within the same file.
        2. For each remaining unique (and filtered) failure location, extracts the
           surrounding log context from its associated snippet using
           `_extract_log_context_internal`.
        3. Formats each processed location into a dictionary:
           `{'file_name': 'filename:line', 'extracted_code': 'context_string'}`.
        4. Sorts the final list of these dictionaries using the static method
           `sort_key_failed_code_path`.

        Args:
            all_raw_failure_data_for_testsuite (list[tuple[str, int, str]]):
                A list of tuples collected from all failed/aborted test cases in a
                testsuite run. Each tuple should be in the format:
                (filename_str, linenumber_int, original_log_snippet_str from the
                testcase where the file:line was found).

        Returns:
            list[dict]:
                A sorted list of dictionaries, ready to be stored. Each dictionary
                represents a significant, unique failure point with its extracted
                log context. Returns an empty list if no processable failures
                are provided or if all are filtered out.
        """
        if not all_raw_failure_data_for_testsuite:
            self.logger.info("No raw failure data provided to process.")
            return []

        self.logger.info(f"Processing {len(all_raw_failure_data_for_testsuite)} raw failure data items for FailedTestCaseCodeCollector.")
        
        # Step 1: Filter nearby locations.
        # Input: [(fname, lnum, snippet), ...]
        # Output: Filtered [(fname, lnum, snippet), ...]
        filtered_tuples_with_snippets = self._filter_nearby_locations_internal(all_raw_failure_data_for_testsuite)
        self.logger.info(f"Filtered down to {len(filtered_tuples_with_snippets)} unique failure locations.")

        # Step 2: Extract context and format for each filtered location
        result_list_of_dicts = []
        seen_locations = set()  # Track unique file:line combinations
        
        if filtered_tuples_with_snippets:
            for fname, lnum, relevant_log_snippet in filtered_tuples_with_snippets:
                file_line_key = f"{fname}:{lnum}"
                
                # Skip if we've already processed this exact location
                if file_line_key in seen_locations:
                    self.logger.debug(f"Skipping duplicate location: {file_line_key}")
                    continue
                    
                seen_locations.add(file_line_key)
                context_string = self._extract_log_context_internal(relevant_log_snippet, fname, lnum)
                
                # Create the entry
                entry = {
                    "file_name": file_line_key,
                    "extracted_code": context_string
                }
                
                # Try to add source code context
                # First, try to get the full path for this file
                full_path = self.find_script_full_path(fname)
                if full_path:
                    source_context = self._extract_code_from_source_file(full_path, lnum)
                    if not source_context.startswith("("):  # Not an error message
                        entry["context_code_from_file"] = source_context
                    else:
                        self.logger.debug(f"Could not extract source context for {file_line_key}: {source_context}")
                
                result_list_of_dicts.append(entry)
        
        # Step 3: Sort the final list of dictionaries
        # Uses the static method of this class for sorting
        sorted_list = sorted(result_list_of_dicts, key=FailedTestCaseCodeCollector.sort_key_failed_code_path)
        
        if sorted_list:
            self.logger.info(f"Successfully processed and formatted {len(sorted_list)} failure code paths with context.")
        else:
            self.logger.info("'failed_code_path' list is empty after all processing.")
            
        return sorted_list
    
    def get_script_path_from_testcases(self, testcases):
        """
        Extracts the script full path from a list of test cases by looking at their failed_code_path data.
        
        Args:
            testcases (list[dict]): List of test case dictionaries that may contain 'failed_code_path' data
            
        Returns:
            str: The full script path if found, empty string otherwise
        """
        for tc in testcases:
            if tc.get("failed_code_path"):
                # Get the first file reference from failed_code_path
                first_failure = tc["failed_code_path"][0]
                if first_failure and "file_name" in first_failure:
                    # Extract just the filename without line number
                    file_ref = first_failure["file_name"]
                    if ":" in file_ref:
                        script_name = file_ref.split(":")[0]
                        # Use the find_script_full_path method to get the full path
                        full_path = self.find_script_full_path(script_name)
                        if full_path:
                            return full_path
        
        self.logger.info("Could not determine script path from test cases")
        return ""