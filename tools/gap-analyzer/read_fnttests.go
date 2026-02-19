package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Base URL for Google AI Gemini Public API
const geminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s"

// --- Structs for parsing metadata.textproto ---

// FNTTest holds metadata about a single feature test
type FNTTest struct {
	ID              string
	Description     string
	RequirementFile string // e.g., [README.md](http://_vscodecontentref_/0)
	AutomationFile  string // e.g., foo_test.go
	TestDir         string // Directory containing metadata.textproto
}

// --- Structs for Gemini Public API Request and Response ---
// These structs are used for JSON marshalling/unmarshalling via [http](http://_vscodecontentref_/1)

// APIPart corresponds to a part of the content request/response.
type APIPart struct {
	Text string `json:"text"`
}

// APIContent corresponds to content in the request/response.
type APIContent struct {
	Role  string    `json:"role,omitempty"`
	Parts []APIPart `json:"parts"`
}

// APISchemaProperty defines a property in the response schema.
type APISchemaProperty struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// APISchema defines the expected JSON response structure.
type APISchema struct {
	Type       string                       `json:"type"`
	Properties map[string]APISchemaProperty `json:"properties,omitempty"`
	Required   []string                     `json:"required,omitempty"`
}

// APIGenerationConfig configures Gemini's output, forcing JSON.
type APIGenerationConfig struct {
	ResponseMimeType string    `json:"responseMimeType"`
	ResponseSchema   APISchema `json:"responseSchema"`
}

// APIRequest is the top-level request body sent to Gemini.
type APIRequest struct {
	Contents         []APIContent        `json:"contents"`
	GenerationConfig APIGenerationConfig `json:"generationConfig"`
}

// APIUsageMetadata contains token count info from the response.
type APIUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// APICandidate contains one potential response from Gemini.
type APICandidate struct {
	Content APIContent `json:"content"`
}

// APIResponse is the top-level response body received from Gemini.
type APIResponse struct {
	Candidates    []APICandidate   `json:"candidates"`
	UsageMetadata APIUsageMetadata `json:"usageMetadata"`
	Error         *struct {        // Field for API-level errors
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GapResult is used to unmarshal the JSON content returned by Gemini.
type GapResult struct {
	GapFound       bool   `json:"gap_found"`
	GapDescription string `json:"gap_description"`
}

// --- Command Line Flags ---
var (
	apiKey              = flag.String("api-key", os.Getenv("GEMINI_API_KEY"), "API Key for Google AI Gemini API. Can also be set via GEMINI_API_KEY env var.")
	model               = flag.String("model", "gemini-2.5-pro", "The public Gemini model to use.")
	featureprofilesRoot = flag.String("featureprofiles-root", ".", "Root directory for searching tests (e.g., '.' for repo root).")
	changedFilesStr     = flag.String("changed-files", "", "Comma-separated list of changed files.")
)

// --- Regular Expressions for parsing metadata.textproto ---
var (
	descRegex = regexp.MustCompile(`description:\s*"([^"]+)"`)
	idRegex   = regexp.MustCompile(`test_plan_id:\s*"([^"]+)"`)
)

// parseMetadata parses a metadata.textproto file to extract test info.
// It uses regex for simplicity, avoiding proto dependencies.
func parseMetadata(path string) (*FNTTest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)

	test := &FNTTest{TestDir: filepath.Dir(path)}

	if match := descRegex.FindStringSubmatch(content); len(match) > 1 {
		test.Description = match[1]
	}
	if match := idRegex.FindStringSubmatch(content); len(match) > 1 {
		test.ID = match[1]
	}

	return test, nil
}

// findFNTTests walks the root directory to find and parse metadata.textproto files.
func findFNTTests(rootDir string) ([]*FNTTest, error) {
	var tests []*FNTTest
	log.Printf("Searching for tests in: %s", rootDir)
	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Base(path) == "metadata.textproto" {
			test, err := parseMetadata(path)
			if err != nil {
				log.Printf("Warning: could not parse %s: %v", path, err)
				return nil // Continue walking even if one metadata is bad
			}
			// If metadata.textproto does not contain test_plan_id, it's not a valid test.
			if test.ID == "" {
				log.Printf("Info: Skipping %s: metadata.textproto lacks test_plan_id.", path)
				return nil
			}

			dirEntries, err := os.ReadDir(test.TestDir)
			if err != nil {
				log.Printf("Warning: could not read dir %s: %v", test.TestDir, err)
				return nil
			}

			for _, entry := range dirEntries {
				if !entry.IsDir() {
					if entry.Name() == "README.md" {
						test.RequirementFile = entry.Name()
					} else if strings.HasSuffix(entry.Name(), ".go") {
						test.AutomationFile = entry.Name()
					}
				}
			}

			if test.RequirementFile != "" && test.AutomationFile != "" {
				tests = append(tests, test)
			} else {
				if test.RequirementFile == "" {
					log.Printf("Warning: skipping test in %s: missing README.md", test.TestDir)
				}
				if test.AutomationFile == "" {
					log.Printf("Warning: skipping test in %s: missing *.go", test.TestDir)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking directory %s: %w", rootDir, err)
	}
	log.Printf("Found %d tests.", len(tests))
	return tests, nil
}

// callGemini sends the readme and automation content to the public Gemini API
// and returns the parsed gap analysis result and token count.
func callGemini(ctx context.Context, readmeContent, automationContent string) (bool, string, int, error) {
	prompt := fmt.Sprintf(`
Preamble: You are a test engineer analyzing test coverage.
Task: Analyze the provided readme markdown and automation code. Identify any gaps in the automation code based on the requirements provided in the readme markdown.
Note that any "TODO" items or sections in the Readme Markdown are not considered requirements and should be ignored when identifying gaps.
Ignore any gap in the automation if the same is covered using a deviation.
Return the result in JSON format with two fields: 'gap_found' (boolean) and 'gap_description' (string).
If gaps are found, 'gap_found' should be true and 'gap_description' should contain a description of what requirements are not covered by the test automation.
If all requirements in the readme are covered, 'gap_found' should be false and 'gap_description' should be 'No gap in the implementation found.'.

Readme Markdown:
---
%s
---

Automation Code:
---
%s
---

Result in JSON format:
`, readmeContent, automationContent)

	// Define schema to force JSON output
	schema := APISchema{
		Type: "OBJECT",
		Properties: map[string]APISchemaProperty{
			"gap_found":       {Type: "BOOLEAN"},
			"gap_description": {Type: "STRING"},
		},
		Required: []string{"gap_found", "gap_description"},
	}

	// Build request body
	reqBody := APIRequest{
		Contents: []APIContent{
			{
				Parts: []APIPart{{Text: prompt}},
			},
		},
		GenerationConfig: APIGenerationConfig{
			ResponseMimeType: "application/json",
			ResponseSchema:   schema,
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return false, "", 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP request
	url := fmt.Sprintf(geminiAPIURL, *model, *apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return false, "", 0, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, "", 0, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", 0, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, "", 0, fmt.Errorf("gemini api returned non-ok status %d: %s", resp.StatusCode, string(respBody))
	}

	// Unmarshal response body
	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return false, "", 0, fmt.Errorf("failed to unmarshal api response: %w", err)
	}

	if apiResp.Error != nil {
		return false, "", 0, fmt.Errorf("gemini api error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 {
		return false, "", 0, fmt.Errorf("gemini returned no candidates in response")
	}

	// Unmarshal the actual gap result JSON from the candidate text
	var gapResult GapResult
	resultText := apiResp.Candidates[0].Content.Parts[0].Text
	if err := json.Unmarshal([]byte(resultText), &gapResult); err != nil {
		return false, "", 0, fmt.Errorf("failed to unmarshal gap result json '%s': %w", resultText, err)
	}

	tokenCount := apiResp.UsageMetadata.TotalTokenCount
	log.Printf("Token usage: %d", tokenCount)

	return gapResult.GapFound, gapResult.GapDescription, tokenCount, nil
}

func main() {
	flag.Parse()
	ctx := context.Background()

	if *apiKey == "" {
		log.Fatal("Error: -api-key flag or GEMINI_API_KEY environment variable must be set.")
	}

	allTests, err := findFNTTests(*featureprofilesRoot)
	if err != nil {
		log.Fatalf("Error finding FNT tests: %v", err)
	}

	// Filter tests to only those with changed files for PR check.
	if *changedFilesStr == "" {
		log.Println("No changed files specified with -changed-files, no tests to run.")
		os.Exit(0)
	}
	changed := make(map[string]bool)
	for _, f := range strings.Split(*changedFilesStr, ",") {
		changed[f] = true
	}

	var testsToRun []*FNTTest
	for _, test := range allTests {
		readmePath := filepath.Join(test.TestDir, test.RequirementFile)
		autoPath := filepath.Join(test.TestDir, test.AutomationFile)
		if changed[readmePath] || changed[autoPath] {
			testsToRun = append(testsToRun, test)
		}
	}
	log.Printf("Found %d tests to analyze based on changed files.", len(testsToRun))

	gapsOrErrorsFound := false
	var failureMessages []string
	var totalTokens int64

	for _, test := range testsToRun {
		readmePath := filepath.Join(test.TestDir, test.RequirementFile)
		autoPath := filepath.Join(test.TestDir, test.AutomationFile)

		log.Printf("Analyzing test: %s", test.ID)

		readmeContent, err := os.ReadFile(readmePath)
		if err != nil {
			log.Printf("Warning: Failed to read readme %s: %v", readmePath, err)
			gapsOrErrorsFound = true
			failureMessages = append(failureMessages, fmt.Sprintf("Error reading %s for test %s: %v", readmePath, test.ID, err))
			continue
		}
		autoContent, err := os.ReadFile(autoPath)
		if err != nil {
			log.Printf("Warning: Failed to read automation %s: %v", autoPath, err)
			gapsOrErrorsFound = true
			failureMessages = append(failureMessages, fmt.Sprintf("Error reading %s for test %s: %v", autoPath, test.ID, err))
			continue
		}

		gapFound, gapDesc, tokens, err := callGemini(ctx, string(readmeContent), string(autoContent))
		totalTokens += int64(tokens)

		if err != nil {
			log.Printf("Warning: Gemini analysis failed for %s: %v", test.ID, err)
			gapsOrErrorsFound = true
			failureMessages = append(failureMessages, fmt.Sprintf("Gemini analysis failed for %s: %v", test.ID, err))
			continue
		}

		if gapFound {
			gapsOrErrorsFound = true
			failureMessages = append(failureMessages, fmt.Sprintf("Gap found in %s: %s", test.ID, gapDesc))
		}
		// // Add sleep to avoid hitting rate limits
		// time.Sleep(2 * time.Second) // Sleep for 2s between tests if required
	}

	// If gaps or errors were found, print messages and exit with failure
	if gapsOrErrorsFound {
		fmt.Println("--- FNT Gap Analysis Failed ---")
		for _, msg := range failureMessages {
			fmt.Println(msg)
		}
		os.Exit(1)
	}

	log.Println("FNT Gap Analysis finished successfully.")
}
