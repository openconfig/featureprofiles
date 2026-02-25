package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestCheckApprovalsLogic(t *testing.T) {
	tests := []struct {
		name          string
		prFiles       []string
		prReviews     []review
		teamMembers   map[string][]string
		codeowners    []Rule
		expectError   bool
		expectedError string
	}{
		{
			name:    "SuccessGlobalApproverOnly",
			prFiles: []string{"README.md"},
			prReviews: []review{
				{Author: struct {
					Login string `json:"login"`
				}{"approver1"}, State: "APPROVED", SubmittedAt: "2023-01-01"},
			},
			teamMembers: map[string][]string{
				"@openconfig/featureprofiles-approvers": {"approver1"},
			},
			codeowners: []Rule{
				{Pattern: "*", Owners: []string{"@openconfig/featureprofiles-approvers"}},
			},
			expectError: false,
		},
		{
			name:      "FailureNoApprovals",
			prFiles:   []string{"README.md"},
			prReviews: []review{},
			teamMembers: map[string][]string{
				"@openconfig/featureprofiles-approvers": {"approver1"},
			},
			codeowners: []Rule{
				{Pattern: "*", Owners: []string{"@openconfig/featureprofiles-approvers"}},
			},
			expectError:   true,
			expectedError: "missing approvals from: [@openconfig/featureprofiles-approvers]",
		},
		{
			name:    "SuccessSpecificFeatureApproval",
			prFiles: []string{"feature/bgp/tests/foo_test.go"},
			prReviews: []review{
				{Author: struct {
					Login string `json:"login"`
				}{"approver1"}, State: "APPROVED", SubmittedAt: "2023-01-01"},
				{Author: struct {
					Login string `json:"login"`
				}{"bgp_owner1"}, State: "APPROVED", SubmittedAt: "2023-01-01"},
			},
			teamMembers: map[string][]string{
				"@openconfig/featureprofiles-approvers": {"approver1"},
				"@openconfig/featureprofiles-owner-bgp": {"bgp_owner1"},
			},
			codeowners: []Rule{
				{Pattern: "*", Owners: []string{"@openconfig/featureprofiles-approvers"}},
				{Pattern: "/feature/bgp/", Owners: []string{"@openconfig/featureprofiles-owner-bgp"}},
			},
			expectError: false,
		},
		{
			name:    "FailureMissingFeatureApproval",
			prFiles: []string{"feature/bgp/tests/foo_test.go"},
			prReviews: []review{
				{Author: struct {
					Login string `json:"login"`
				}{"approver1"}, State: "APPROVED", SubmittedAt: "2023-01-01"},
			},
			teamMembers: map[string][]string{
				"@openconfig/featureprofiles-approvers": {"approver1"},
				"@openconfig/featureprofiles-owner-bgp": {"bgp_owner1"},
			},
			codeowners: []Rule{
				{Pattern: "*", Owners: []string{"@openconfig/featureprofiles-approvers"}},
				{Pattern: "/feature/bgp/", Owners: []string{"@openconfig/featureprofiles-owner-bgp"}},
			},
			expectError:   true,
			expectedError: "missing approvals from: [@openconfig/featureprofiles-owner-bgp]",
		},
		{
			name:    "FailureRevokedApprovalChangesRequested",
			prFiles: []string{"README.md"},
			prReviews: []review{
				{Author: struct {
					Login string `json:"login"`
				}{"approver1"}, State: "APPROVED", SubmittedAt: "2023-01-01"},
				{Author: struct {
					Login string `json:"login"`
				}{"approver1"}, State: "CHANGES_REQUESTED", SubmittedAt: "2023-01-02"},
			},
			teamMembers: map[string][]string{
				"@openconfig/featureprofiles-approvers": {"approver1"},
			},
			codeowners: []Rule{
				{Pattern: "*", Owners: []string{"@openconfig/featureprofiles-approvers"}},
			},
			expectError:   true,
			expectedError: "missing approvals from: [@openconfig/featureprofiles-approvers]",
		},
		{
			name:    "SuccessReApprovalAfterChangesRequested",
			prFiles: []string{"README.md"},
			prReviews: []review{
				{Author: struct {
					Login string `json:"login"`
				}{"approver1"}, State: "APPROVED", SubmittedAt: "2023-01-01"},
				{Author: struct {
					Login string `json:"login"`
				}{"approver1"}, State: "CHANGES_REQUESTED", SubmittedAt: "2023-01-02"},
				{Author: struct {
					Login string `json:"login"`
				}{"approver1"}, State: "APPROVED", SubmittedAt: "2023-01-03"},
			},
			teamMembers: map[string][]string{
				"@openconfig/featureprofiles-approvers": {"approver1"},
			},
			codeowners: []Rule{
				{Pattern: "*", Owners: []string{"@openconfig/featureprofiles-approvers"}},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := func(args ...string) (string, error) {
				if len(args) < 3 {
					return "", fmt.Errorf("unexpected args: %v", args)
				}
				cmd := args[0]
				subcmd := args[1]

				if cmd == "gh" && subcmd == "pr" && args[2] == "view" {
					if args[4] == "--json" && args[5] == "files" {
						// Mock files
						files := struct {
							Files []struct {
								Path string `json:"path"`
							} `json:"files"`
						}{}
						for _, p := range tt.prFiles {
							files.Files = append(files.Files, struct {
								Path string `json:"path"`
							}{Path: p})
						}
						out, _ := json.Marshal(files)
						return string(out), nil
					}
					if args[4] == "--json" && args[5] == "reviews" {
						// Mock reviews
						reviewsStruct := struct {
							Reviews []review `json:"reviews"`
						}{Reviews: tt.prReviews}
						out, _ := json.Marshal(reviewsStruct)
						return string(out), nil
					}
				}

				if cmd == "gh" && subcmd == "api" {
					// Mock team members
					// Format: orgs/%s/teams/%s/members
					urlParts := strings.Split(args[2], "/")
					if len(urlParts) >= 4 {
						org := urlParts[1]
						slug := urlParts[3]
						teamSlug := fmt.Sprintf("@%s/%s", org, slug)

						members, ok := tt.teamMembers[teamSlug]
						if !ok {
							// Try without @ if needed or check input
							// For simplicity, assume exact match in map
							return "", nil
						}

						// The real code uses --jq '.[].login' which returns raw strings, one per line (roughly)
						// The real code expects: "user1\nuser2"

						var sb strings.Builder
						for _, m := range members {
							sb.WriteString(m + "\n")
						}
						return sb.String(), nil
					}
				}

				return "", fmt.Errorf("unexpected command: %v", args)
			}

			err := checkApprovalsLogic(runner, "123", tt.codeowners)
			if tt.expectError {
				if err == nil {
					t.Errorf("checkApprovalsLogic(..., %v) got nil, want error %q", tt.codeowners, tt.expectedError)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("checkApprovalsLogic(..., %v) got error %q, want error containing %q", tt.codeowners, err.Error(), tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("checkApprovalsLogic(..., %v) got error %v, want nil", tt.codeowners, err)
				}
			}
		})
	}
}