import os
from datetime import datetime
import requests
import re
import logging
from dotenv import load_dotenv  

load_dotenv()
logger = logging.getLogger(__name__)

CISCO_URL_BASE = os.environ.get("CISCO_URL_BASE")
PUBLIC_URL_BASE = os.environ.get("PUBLIC_URL_BASE")

class Github:
    def __init__(self):
        # Get GitHub tokens from environment variables
        self.public_token = os.environ.get("PUBLIC_GITHUB_TOKEN")
        self.cisco_token = os.environ.get("CISCO_GITHUB_TOKEN") 
        
    def is_open(self, name):
        """Determine if Github Issue is Open. Later to be used for dynamic inheritance."""
        logger.debug(f"is_open: Checking status for GitHub item: {name}")
        try:
            item_data = self._get_github_data(name)
            if item_data:
                if self._is_pull_request(name):
                    # For PRs, consider merged as closed
                    status = not (item_data.get("merged") or item_data.get("state") == "closed")
                    logger.debug(f"is_open: PR '{name}' merged: {item_data.get('merged')}, state: {item_data.get('state')}. Is open: {status}")
                    return status
                else:
                    # For issues
                    status = item_data.get("state") == "open"
                    logger.debug(f"is_open: Issue '{name}' state: {item_data.get('state')}. Is open: {status}")
                    return status
            logger.warning(f"is_open: Could not get GitHub data for '{name}'. Defaulting to True (open).")
            return True  # Default to True if we can't determine
        except Exception as e:
            logger.warning(f"is_open: Error checking if GitHub item is open for '{name}': {e}", exc_info=True)
            return True  # Default to True on error
    
    def _parse_github_url(self, url):
        """Parse GitHub URL to extract owner, repo, type, and number."""
        logger.debug(f"_parse_github_url: Attempting to parse URL: {url}")
        if not url:
            logger.debug("_parse_github_url: URL is empty.")
            return None
        
        # Skip common placeholder texts
        if url.lower() in ["new failure. todo", "issue after upstream merge", 
                          "need fix new test version"]:
            logger.info(f"_parse_github_url: Skipping placeholder text: {url}")
            return None
            
        # Match pattern for both public and Cisco internal GitHub URLs
        pattern = r"https?://((?:wwwin-)?github\.cisco\.com|github\.com)/([^/]+)/([^/]+)/(issues|pull|tree)/([^/]*)?"
        match = re.search(pattern, url)
        
        if match:
            domain, owner, repo, item_type, number_or_branch = match.groups()
            result = {
                "owner": owner,
                "repo": repo,
                "type": item_type,
                "is_cisco": "cisco.com" in domain
            }
            if item_type in ["pull", "issues"] and number_or_branch and number_or_branch.isdigit():
                result["number"] = number_or_branch
            elif item_type == "tree":
                # For tree URLs, we use the branch name
                result["branch"] = number_or_branch or "unknown"
            else:
                logger.warning(f"_parse_github_url: Unrecognized item type or number format for {url}. Groups: {match.groups()}")
                return None
            logger.debug(f"_parse_github_url: Successfully parsed URL: {result}") # Changed url_info to result here
            return result
    
        logger.warning(f"_parse_github_url: URL format not recognized for: {url}")
        return None
    
    def _is_pull_request(self, url):
        """Determine if URL is for a pull request."""
        logger.debug(f"_is_pull_request: Checking if '{url}' is a pull request.")
        url_info = self._parse_github_url(url)
        is_pr = url_info and url_info.get("type") == "pull"
        logger.debug(f"_is_pull_request: '{url}' is PR: {is_pr}")
        return is_pr
    
    def _get_github_data(self, url):
        """Get data for a GitHub issue or PR using the GitHub API."""
        logger.debug(f"_get_github_data: Attempting to fetch data for URL: {url}")
        url_info = self._parse_github_url(url)
        if not url_info:
            logger.warning(f"_get_github_data: Could not parse GitHub URL: {url}. Cannot fetch data.")
            return None
            
        if url_info.get("type") == "tree":
            logger.debug(f"_get_github_data: Skipping API call for tree URL: {url}.")
            return None
            
        if "number" not in url_info:
            logger.warning(f"_get_github_data: Missing issue/PR number for URL: {url}. Cannot fetch data.")
            return None
            
        api_base = CISCO_URL_BASE if url_info.get("is_cisco", False) else PUBLIC_URL_BASE
        token = self.cisco_token if url_info.get("is_cisco", False) else self.public_token
        
        api_url = f"{api_base}/repos/{url_info['owner']}/{url_info['repo']}/{'pulls' if url_info['type'] == 'pull' else 'issues'}/{url_info['number']}"
        
        headers = {}
        if token:
            headers["Authorization"] = f"token {token}"
            
        try:
            logger.debug(f"_get_github_data: Making API request to: {api_url}")
            response = requests.get(api_url, headers=headers)
            logger.debug(f"_get_github_data: API response status: {response.status_code}")
            if response.status_code == 200:
                return response.json()
            else:
                logger.warning(f"_get_github_data: GitHub API request failed: {response.status_code} for URL {api_url}. Response text: {response.text[:200]}...")
                return None
        except requests.exceptions.RequestException as e:
            logger.error(f"_get_github_data: Network/Request Error fetching GitHub data for {api_url}: {e}", exc_info=True)
            return None
        except Exception as e:
            logger.error(f"_get_github_data: Unexpected Error fetching GitHub data for {api_url}: {e}", exc_info=True)
            return None
    
    def inherit(self, name):
        """
        Create a Github bug to inherit with additional information.
        Will always return a bug object if the URL is valid (not a placeholder or unrecognized format),
        even if API data cannot be fetched.
        """
        logger.debug(f"inherit: Attempting to inherit GitHub bug: {name}")
        url_info = self._parse_github_url(name)
        
        # Only return None if URL is unparseable or a known placeholder
        if not url_info:
            logger.info(f"inherit: Skipping GitHub bug: URL '{name}' could not be parsed or is a placeholder.")
            return None
        
        # Base bug object
        bug = {
            "name": name,
            "type": "Github",
            "username": "Cisco InstaTriage",
            "updated": datetime.now(),
            "resolved": not self.is_open(name) # resolved state depends on is_open
        }
        
        item_data = self._get_github_data(name) # This might be None if API call fails or for 'tree' URLs
        
        if item_data: # Only use item_data if it was successfully fetched
            github_type = "PR" if self._is_pull_request(name) else "Issue"
            bug["github_type"] = github_type
            if "user" in item_data and "login" in item_data["user"]:
                bug["username"] = item_data["user"]["login"]
            if "title" in item_data:
                bug["headline"] = item_data["title"]
            if "created_at" in item_data:
                try:
                    created_at = datetime.strptime(item_data["created_at"], "%Y-%m-%dT%H:%M:%SZ")
                    age_days = (datetime.now() - created_at).days
                    bug["age"] = age_days
                except Exception as e:
                    logger.warning(f"inherit: Error calculating GitHub age for '{name}': {e}")
            if github_type == "PR":
                if item_data.get("merged"):
                    bug["github_status"] = "Merged"
                elif item_data.get("state") == "closed":
                    bug["github_status"] = "Closed"
                else:
                    bug["github_status"] = "Open"
            else: # Issue
                bug["github_status"] = item_data.get("state", "Unknown").capitalize()
            logger.debug(f"inherit: Populated bug for '{name}' with API data. Bug status: {bug.get('github_status')}, Resolved: {bug.get('resolved')}")
        else:
            # Provide default values if API data is not available
            logger.warning(f"inherit: GitHub API data not available for '{name}'. Populating with default status.")
            bug["github_type"] = url_info.get("type", "Unknown").capitalize()
            bug["headline"] = f"GitHub {bug['github_type']} (API data unavailable)"
            bug["github_status"] = "API_Unavailable" # More descriptive default status
            bug["age"] = "N/A" # Default age
            logger.debug(f"inherit: Populated bug for '{name}' with default data. Bug status: {bug.get('github_status')}, Resolved: {bug.get('resolved')}")
            
        return bug