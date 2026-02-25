import json
import sys

def check_approvals(reviews_json):
    reviews = json.loads(reviews_json)
    # Simulate: [.reviews[] | select(.state == "APPROVED")] | unique_by(.author.login) | length
    approved_reviews = [r for r in reviews.get("reviews", []) if r.get("state") == "APPROVED"]
    unique_approvers = set()
    for r in approved_reviews:
        login = r.get("author", {}).get("login")
        if login:
            unique_approvers.add(login)
    
    count = len(unique_approvers)
    print(f"Found {count} unique approval(s).")
    return count >= 1

def run_tests():
    # Test Case 1: No reviews
    print("Test Case 1: No reviews")
    json1 = '{"reviews": []}'
    assert not check_approvals(json1), "Failed: Expected no approvals"
    print("Passed")

    # Test Case 2: One approval
    print("\nTest Case 2: One approval")
    json2 = '{"reviews": [{"author": {"login": "user1"}, "state": "APPROVED"}]}'
    assert check_approvals(json2), "Failed: Expected one approval"
    print("Passed")

    # Test Case 3: Multiple approvals, unique users
    print("\nTest Case 3: Multiple approvals")
    json3 = '{"reviews": [{"author": {"login": "user1"}, "state": "APPROVED"}, {"author": {"login": "user2"}, "state": "APPROVED"}]}'
    assert check_approvals(json3), "Failed: Expected approvals"
    print("Passed")

    # Test Case 4: Duplicate approvals from same user
    print("\nTest Case 4: Duplicate approvals")
    json4 = '{"reviews": [{"author": {"login": "user1"}, "state": "APPROVED"}, {"author": {"login": "user1"}, "state": "APPROVED"}]}'
    assert check_approvals(json4), "Failed: Expected approval (counted once)"
    # Logic check: count should be 1
    # Based on unique_by(.author.login), user1 is counted once.
    print("Passed")

    # Test Case 5: Approval then Changes Requested (Potential Issue)
    print("\nTest Case 5: Approval then Changes Requested")
    json5 = '{"reviews": [{"author": {"login": "user1"}, "state": "APPROVED", "submittedAt": "2023-01-01"}, {"author": {"login": "user1"}, "state": "CHANGES_REQUESTED", "submittedAt": "2023-01-02"}]}'
    is_approved = check_approvals(json5)
    if is_approved:
        print("Note: Logic counts approval even if followed by changes requested.")
    else:
        print("Note: Logic correctly handles revoked approval.")
    
    # Current implementation will print "Found 1 unique approval(s)." and pass.

if __name__ == "__main__":
    run_tests()