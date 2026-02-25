// Package main checks if a PR has all required approvals based on CODEOWNERS.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// CommandRunner is a function type for running shell commands.
type CommandRunner func(args ...string) (string, error)

// realRunCommand is the actual implementation using os/exec.
func realRunCommand(args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("command failed: %s\nstderr: %s", err, exitErr.Stderr)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// prFiles represents the JSON structure for PR files from GitHub CLI.
type prFiles struct {
	Files []struct {
		Path string `json:"path"`
	} `json:"files"`
}

// review represents a single review on a PR.
type review struct {
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	State       string `json:"state"`
	SubmittedAt string `json:"submittedAt"`
}

// prReviews represents the JSON structure for PR reviews from GitHub CLI.
type prReviews struct {
	Reviews []review `json:"reviews"`
}

// Rule represents a single line in the CODEOWNERS file.
type Rule struct {
	Pattern string
	Owners  []string
}

func fetchPRData(runner CommandRunner, prNumber string) ([]string, map[string]bool, error) {
	// Get files
	out, err := runner("gh", "pr", "view", prNumber, "--json", "files")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get PR files: %v", err)
	}
	var filesData prFiles
	if err := json.Unmarshal([]byte(out), &filesData); err != nil {
		return nil, nil, fmt.Errorf("failed to parse files JSON: %v", err)
	}
	var files []string
	for _, f := range filesData.Files {
		files = append(files, f.Path)
	}

	// Get reviews
	out, err = runner("gh", "pr", "view", prNumber, "--json", "reviews")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get PR reviews: %v", err)
	}
	var reviewsData prReviews
	if err := json.Unmarshal([]byte(out), &reviewsData); err != nil {
		return nil, nil, fmt.Errorf("failed to parse reviews JSON: %v", err)
	}

	// Track latest state per reviewer
	latestReviews := make(map[string]review)
	for _, r := range reviewsData.Reviews {
		user := r.Author.Login
		if existing, ok := latestReviews[user]; !ok || r.SubmittedAt > existing.SubmittedAt {
			latestReviews[user] = r
		}
	}

	approvers := make(map[string]bool)
	for user, r := range latestReviews {
		if r.State == "APPROVED" {
			approvers[user] = true
		}
	}

	return files, approvers, nil
}

func parseCodeowners(path string) ([]Rule, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return parseCodeownersReader(bufio.NewScanner(file))
}

func parseCodeownersReader(scanner *bufio.Scanner) ([]Rule, error) {
	var rules []Rule
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		pattern := parts[0]
		owners := parts[1:]
		rules = append(rules, Rule{Pattern: pattern, Owners: owners})
	}
	return rules, scanner.Err()
}

func matchOwners(filePath string, rules []Rule) []string {
	var matchedOwners []string
	for _, rule := range rules {
		pattern := rule.Pattern
		owners := rule.Owners
		isMatch := false

		if strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") {
			prefix := pattern[1:]
			if strings.HasPrefix(filePath, prefix) {
				isMatch = true
			}
		} else if pattern == "*" {
			isMatch = true
		} else if strings.HasPrefix(pattern, "/") && filePath == pattern[1:] {
			isMatch = true
		}

		if isMatch {
			matchedOwners = owners
		}
	}
	return matchedOwners
}

func fetchTeamMembers(runner CommandRunner, teamSlug string) (map[string]bool, error) {
	if !strings.Contains(teamSlug, "/") {
		return map[string]bool{strings.ReplaceAll(teamSlug, "@", ""): true}, nil
	}

	parts := strings.Split(teamSlug, "/")
	org := parts[0]
	slug := parts[1]
	if strings.HasPrefix(org, "@") {
		org = org[1:]
	}

	log.Printf("Fetching members for %s/%s...", org, slug)
	out, err := runner("gh", "api", fmt.Sprintf("orgs/%s/teams/%s/members", org, slug), "--paginate", "--jq", ".[].login")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch team members: %v", err)
	}

	members := make(map[string]bool)
	for _, m := range strings.Split(out, "\n") {
		if m != "" {
			members[m] = true
		}
	}
	return members, nil
}

// checkApprovalsLogic performs the core logic, decoupled from OS/Env.
func checkApprovalsLogic(runner CommandRunner, prNumber string, rules []Rule) error {
	log.Printf("Checking approvals for PR #%s", prNumber)
	files, approvers, err := fetchPRData(runner, prNumber)
	if err != nil {
		return fmt.Errorf("error getting PR data: %v", err)
	}

	approverList := make([]string, 0, len(approvers))
	for k := range approvers {
		approverList = append(approverList, k)
	}
	log.Printf("Approvers found: %v", approverList)

	requiredTeams := make(map[string]bool)
	requiredTeams["@openconfig/featureprofiles-approvers"] = true

	for _, f := range files {
		owners := matchOwners(f, rules)
		for _, owner := range owners {
			if owner != "@openconfig/featureprofiles-approvers" {
				requiredTeams[owner] = true
			}
		}
	}

	requiredTeamsList := make([]string, 0, len(requiredTeams))
	for k := range requiredTeams {
		requiredTeamsList = append(requiredTeamsList, k)
	}
	log.Printf("Required Teams: %v", requiredTeamsList)

	var missingApprovals []string
	teamMembersCache := make(map[string]map[string]bool)

	for team := range requiredTeams {
		if !strings.HasPrefix(team, "@openconfig/") {
			user := strings.ReplaceAll(team, "@", "")
			if !approvers[user] {
				missingApprovals = append(missingApprovals, team)
			}
			continue
		}

		if _, ok := teamMembersCache[team]; !ok {
			members, err := fetchTeamMembers(runner, team)
			if err != nil {
				log.Printf("Warning: %v", err)
				missingApprovals = append(missingApprovals, team)
				continue
			}
			teamMembersCache[team] = members
		}

		members := teamMembersCache[team]
		if len(members) == 0 {
			log.Printf("Warning: No members found for %s. Skipping check implies fail.", team)
			missingApprovals = append(missingApprovals, team)
			continue
		}

		hasApproval := false
		for member := range members {
			if approvers[member] {
				hasApproval = true
				break
			}
		}
		if !hasApproval {
			missingApprovals = append(missingApprovals, team)
		}
	}

	if len(missingApprovals) > 0 {
		return fmt.Errorf("missing approvals from: %v", missingApprovals)
	}

	log.Println("Success: All required approvals satisfied.")
	return nil
}

func main() {
	prNumber := os.Getenv("PR_NUMBER")
	if prNumber == "" {
		log.Fatal("PR_NUMBER env var not set")
	}

	rules, err := parseCodeowners(".github/CODEOWNERS")
	if err != nil {
		log.Printf("CODEOWNERS not found or error reading: %v", err)
	}

	if err := checkApprovalsLogic(realRunCommand, prNumber, rules); err != nil {
		log.Fatal(err)
	}
}