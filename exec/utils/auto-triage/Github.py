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
        """Determine if Github Issue is Open."""
        try:
            item_data = self._get_github_data(name)
            if item_data:
                if self._is_pull_request(name):
                    # For PRs, consider merged as closed
                    return not (item_data.get("merged") or item_data.get("state") == "closed")
                else:
                    # For issues
                    return item_data.get("state") == "open"
            return True  # Default to True if we can't determine 
        except Exception as e:
            logger.warning(f"Error checking if GitHub item is open: {e}")
            return True  # Default to True on error 
    
    def _parse_github_url(self, url):
        """Parse GitHub URL to extract owner, repo, type, and number."""
        if not url:
            return None
        
        # Skip common placeholder texts
        if url.lower() in ["new failure. todo", "issue after upstream merge", 
                          "need fix new test version"]:
            
            logger.info(f"Skipping placeholder text: {url}")
            return None
            
        # Match pattern for both public and Cisco internal GitHub URLs
        pattern = r"https?://((?:wwwin-)?github\.cisco\.com|github\.com)/([^/]+)/([^/]+)/(issues|pull|tree)/([^/]*)?"
        match = re.search(pattern, url)
        
        if match:
            domain, owner, repo, item_type, number_or_branch = match.groups()
            
            # Common object to return regardless of URL type
            result = {
                "owner": owner,
                "repo": repo,
                "type": item_type,
                "is_cisco": "cisco.com" in domain
            }
            
            # Handle different URL types
            if item_type in ["pull", "issues"] and number_or_branch and number_or_branch.isdigit():
                result["number"] = number_or_branch
            elif item_type == "tree":
                # For tree URLs, we use the branch name
                result["branch"] = number_or_branch or "unknown"
            else:
                logger.warning(f"Unrecognized URL format for {url}")
                return None
                
            return result
    
        logger.warning(f"URL format not recognized: {url}")
        return None
    
    def _is_pull_request(self, url):
        """Determine if URL is for a pull request."""
        url_info = self._parse_github_url(url)
        return url_info and url_info.get("type") == "pull"
    
    def _get_github_data(self, url):
        """Get data for a GitHub issue or PR using the GitHub API."""
        url_info = self._parse_github_url(url)
        if not url_info:
            logger.warning(f"Could not parse GitHub URL: {url}")
            return None
            
        # Handle tree URLs differently (they don't have a direct API endpoint)
        if url_info.get("type") == "tree":
            return None  # Return None for tree URLs to skip them
            
        # Verify we have a number for an issue or PR
        if "number" not in url_info:
            logger.warning(f"Missing issue/PR number for URL: {url}")
            return None
            
        # Determine which API base URL and token to use
        if url_info.get("is_cisco", False):
            api_base = CISCO_URL_BASE
            token = self.cisco_token
        else:
            api_base = PUBLIC_URL_BASE
            token = self.public_token
        
        api_url = f"{api_base}/repos/{url_info['owner']}/{url_info['repo']}/{'pulls' if url_info['type'] == 'pull' else 'issues'}/{url_info['number']}"
        
        headers = {}
        if token:
            headers["Authorization"] = f"token {token}"
            
        try:
            response = requests.get(api_url, headers=headers)
            if response.status_code == 200:
                return response.json()
            else:
                logger.warning(f"GitHub API request failed: {response.status_code} for URL {api_url}")
                return None
        except Exception as e:
            logger.error(f"Error fetching GitHub data: {e}")
            return None
    
    def inherit(self, name):
        """
        Create a Github bug to inherit with additional information.
        Returns None for tree/branch URLs since they can't be properly tracked.
        """
        # Parse the URL first to determine if it's a tree/branch URL
        url_info = self._parse_github_url(name)
        
        # If it's a tree/branch URL or couldn't be parsed, return None to skip tracking
        if not url_info or url_info.get("type") == "tree":
            logger.info(f"Skipping GitHub branch/tree URL: {name}")
            return None
        
        # Base bug object (same as before)
        bug = {
            "name": name,
            "type": "Github",
            "username": "Cisco InstaTriage",
            "updated": datetime.now(),
            "resolved": not self.is_open(name)
        }
        
        # Try to get additional data from GitHub API
        try:
            item_data = self._get_github_data(name)
            if item_data:
                # Determine if it's a PR or Issue
                github_type = "PR" if self._is_pull_request(name) else "Issue"
                bug["github_type"] = github_type
                
                # Get submitter (overwrite the default username)
                if "user" in item_data and "login" in item_data["user"]:
                    bug["username"] = item_data["user"]["login"]
                
                # Get title/headline
                if "title" in item_data:
                    bug["headline"] = item_data["title"]
                
                # Calculate age in days
                if "created_at" in item_data:
                    try:
                        created_at = datetime.strptime(item_data["created_at"], "%Y-%m-%dT%H:%M:%SZ")
                        age_days = (datetime.now() - created_at).days
                        bug["age"] = age_days
                    except Exception as e:
                        logger.warning(f"Error calculating GitHub age: {e}")
                
                # Set status
                if github_type == "PR":
                    if item_data.get("merged"):
                        bug["github_status"] = "Merged"
                    elif item_data.get("state") == "closed":
                        bug["github_status"] = "Closed"
                    else:
                        bug["github_status"] = "Open"
                else:  # Issue
                    bug["github_status"] = item_data.get("state", "Unknown").capitalize()
        except Exception as e:
            logger.error(f"Error getting additional GitHub data: {e}")
            
        return bug