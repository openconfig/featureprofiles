import os
import sys
import json
from unittest.mock import MagicMock, patch

# --- Logic from the workflow (adapted for testing) ---

def get_pr_data(pr_number, mock_gh_files, mock_gh_reviews):
    # Mocking gh command output
    files = [f['path'] for f in json.loads(mock_gh_files)['files']]
    reviews = json.loads(mock_gh_reviews)['reviews']
    approvers = {r['author']['login'] for r in reviews if r['state'] == 'APPROVED'}
    return files, approvers

def parse_codeowners(codeowners_content):
    rules = []
    for line in codeowners_content.splitlines():
        line = line.strip()
        if not line or line.startswith('#'):
            continue
        parts = line.split()
        pattern = parts[0]
        owners = parts[1:]
        rules.append((pattern, owners))
    return rules

def match_owners(file_path, rules):
    matched_owners = []
    for pattern, owners in rules:
        is_match = False
        if pattern.startswith('/') and pattern.endswith('/'):
            prefix = pattern[1:]
            if file_path.startswith(prefix):
                is_match = True
        elif pattern == '*':
            is_match = True
        elif pattern.startswith('/') and file_path == pattern[1:]:
            is_match = True
        
        if is_match:
            matched_owners = owners
    return matched_owners

def check_approvals(pr_number, mock_files_json, mock_reviews_json, mock_team_members, codeowners_content):
    print(f"--- Checking PR #{pr_number} ---")
    files, approvers = get_pr_data(pr_number, mock_files_json, mock_reviews_json)
    print(f"Files modified: {files}")
    print(f"Approvers found: {approvers}")
    
    rules = parse_codeowners(codeowners_content)
    
    required_teams = set()
    required_teams.add("@openconfig/featureprofiles-approvers")
    
    for f in files:
        owners = match_owners(f, rules)
        for owner in owners:
            if owner != "@openconfig/featureprofiles-approvers":
                required_teams.add(owner)
                
    print(f"Required Teams: {required_teams}")
    
    missing_approvals = []
    
    for team in required_teams:
        if not team.startswith('@openconfig/'):
             user = team.replace('@', '')
             if user not in approvers:
                 missing_approvals.append(team)
             continue

        members = mock_team_members.get(team, set())
        if not members:
            print(f"Warning: No members found for {team}")
            missing_approvals.append(team)
            continue

        has_approval = not approvers.isdisjoint(members)
        if not has_approval:
            missing_approvals.append(team)
    
    if missing_approvals:
        print(f"FAILURE: Missing approvals from: {missing_approvals}")
        return False
    
    print("SUCCESS: All required approvals satisfied.")
    return True

# --- Test Data & Execution ---

def run_tests():
    # Mock CODEOWNERS content
    codeowners_content = """
*       @openconfig/featureprofiles-approvers
/feature/bgp/          @openconfig/featureprofiles-owner-bgp
/feature/acl/          @openconfig/featureprofiles-owner-acl
    """
    
    # Mock Team Members
    mock_team_members = {
        "@openconfig/featureprofiles-approvers": {"approver1", "approver2"},
        "@openconfig/featureprofiles-owner-bgp": {"bgp_owner1", "bgp_owner2"},
        "@openconfig/featureprofiles-owner-acl": {"acl_owner1"},
    }

    # Test Case 1: Simple change, global approver only
    print("\nTest 1: README.md change (requires global approver)")
    check_approvals(
        pr_number="1",
        mock_files_json='{"files": [{"path": "README.md"}]}',
        mock_reviews_json='{"reviews": [{"author": {"login": "approver1"}, "state": "APPROVED"}]}',
        mock_team_members=mock_team_members,
        codeowners_content=codeowners_content
    )

    # Test Case 2: BGP change, missing BGP owner approval
    print("\nTest 2: BGP change (missing BGP owner)")
    check_approvals(
        pr_number="2",
        mock_files_json='{"files": [{"path": "feature/bgp/tests/foo_test.go"}]}',
        mock_reviews_json='{"reviews": [{"author": {"login": "approver1"}, "state": "APPROVED"}]}',
        mock_team_members=mock_team_members,
        codeowners_content=codeowners_content
    )

    # Test Case 3: BGP change, has BGP owner approval but missing global
    print("\nTest 3: BGP change (missing global approver)")
    check_approvals(
        pr_number="3",
        mock_files_json='{"files": [{"path": "feature/bgp/tests/foo_test.go"}]}',
        mock_reviews_json='{"reviews": [{"author": {"login": "bgp_owner1"}, "state": "APPROVED"}]}',
        mock_team_members=mock_team_members,
        codeowners_content=codeowners_content
    )
    
    # Test Case 4: BGP change, has both approvals (separate people)
    print("\nTest 4: BGP change (has both approvals)")
    check_approvals(
        pr_number="4",
        mock_files_json='{"files": [{"path": "feature/bgp/tests/foo_test.go"}]}',
        mock_reviews_json='{"reviews": [{"author": {"login": "approver1"}, "state": "APPROVED"}, {"author": {"login": "bgp_owner1"}, "state": "APPROVED"}]}',
        mock_team_members=mock_team_members,
        codeowners_content=codeowners_content
    )
    
    # Test Case 5: BGP change, has one person who is in BOTH teams
    # (Assuming intersection is possible, though not in my mock data above. Let's add one)
    mock_team_members["@openconfig/featureprofiles-approvers"].add("super_user")
    mock_team_members["@openconfig/featureprofiles-owner-bgp"].add("super_user")
    
    print("\nTest 5: BGP change (one person in both teams)")
    check_approvals(
        pr_number="5",
        mock_files_json='{"files": [{"path": "feature/bgp/tests/foo_test.go"}]}',
        mock_reviews_json='{"reviews": [{"author": {"login": "super_user"}, "state": "APPROVED"}]}',
        mock_team_members=mock_team_members,
        codeowners_content=codeowners_content
    )

if __name__ == "__main__":
    run_tests()