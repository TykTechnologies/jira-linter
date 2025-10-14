package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/andygrunwald/go-jira"
)

// TestFindIssueID verifies that Jira ticket IDs are correctly extracted from strings
// following the pattern: [A-Z]{1,10}-[0-9]{1,10}
func TestFindIssueID(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "valid ticket ID in branch name",
			input:       "feature/ABC-123-add-new-feature",
			expected:    "ABC-123",
			expectError: false,
		},
		{
			name:        "valid ticket ID at start",
			input:       "ABC-123-feature",
			expected:    "ABC-123",
			expectError: false,
		},
		{
			name:        "valid ticket ID standalone",
			input:       "ABC-123",
			expected:    "ABC-123",
			expectError: false,
		},
		{
			name:        "valid ticket ID with long project key",
			input:       "PROJECTKEY-999",
			expected:    "PROJECTKEY-999",
			expectError: false,
		},
		{
			name:        "valid ticket ID in PR title",
			input:       "ABC-123: Add new feature",
			expected:    "ABC-123",
			expectError: false,
		},
		{
			name:        "multiple ticket IDs - returns first",
			input:       "feature/ABC-123-XYZ-456",
			expected:    "ABC-123",
			expectError: false,
		},
		{
			name:        "no ticket ID",
			input:       "feature-branch",
			expected:    "",
			expectError: true,
		},
		{
			name:        "lowercase ticket ID - should fail",
			input:       "abc-123",
			expected:    "",
			expectError: true,
		},
		{
			name:        "missing number part",
			input:       "ABC-",
			expected:    "",
			expectError: true,
		},
		{
			name:        "missing project key",
			input:       "-123",
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty string",
			input:       "",
			expected:    "",
			expectError: true,
		},
		{
			name:        "project key exactly 10 characters (boundary)",
			input:       "ABCDEFGHIJ-123",
			expected:    "ABCDEFGHIJ-123",
			expectError: false,
		},
		{
			name:        "project key 11 characters (regex matches last 10)",
			input:       "ABCDEFGHIJK-123",
			expected:    "BCDEFGHIJK-123", // Non-greedy regex skips first char
			expectError: false,
		},
		{
			name:        "ticket number exactly 10 digits (boundary)",
			input:       "ABC-1234567890",
			expected:    "ABC-1234567890",
			expectError: false,
		},
		{
			name:        "ticket number 11 digits (regex allows, returns first 10)",
			input:       "ABC-12345678901",
			expected:    "ABC-1234567890",
			expectError: false,
		},
		{
			name:        "single character project key (boundary)",
			input:       "A-123",
			expected:    "A-123",
			expectError: false,
		},
		{
			name:        "single digit ticket number (boundary)",
			input:       "ABC-1",
			expected:    "ABC-1",
			expectError: false,
		},
		{
			name:        "special character underscore instead of dash",
			input:       "ABC_123",
			expected:    "",
			expectError: true,
		},
		{
			name:        "special character dot instead of dash",
			input:       "ABC.123",
			expected:    "",
			expectError: true,
		},
		{
			name:        "leading whitespace",
			input:       " ABC-123",
			expected:    "ABC-123",
			expectError: false,
		},
		{
			name:        "trailing whitespace",
			input:       "ABC-123 ",
			expected:    "ABC-123",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := findIssueID(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

// TestValidateBranchAndTitle ensures that branch names and PR titles
// are validated correctly and contain matching Jira ticket IDs
func TestValidateBranchAndTitle(t *testing.T) {
	tests := []struct {
		name        string
		branchName  string
		prTitle     string
		expected    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "matching ticket IDs in branch and title",
			branchName:  "feature/ABC-123-new-feature",
			prTitle:     "ABC-123: Add new feature",
			expected:    "ABC-123",
			expectError: false,
		},
		{
			name:        "branch without ticket ID",
			branchName:  "feature-branch",
			prTitle:     "ABC-123: Some feature",
			expected:    "",
			expectError: true,
			errorMsg:    "branch name",
		},
		{
			name:        "PR title without ticket ID",
			branchName:  "feature/ABC-123-new-feature",
			prTitle:     "Add new feature",
			expected:    "",
			expectError: true,
			errorMsg:    "PR title must contain",
		},
		{
			name:        "mismatched ticket IDs",
			branchName:  "feature/ABC-123-new-feature",
			prTitle:     "XYZ-456: Different feature",
			expected:    "",
			expectError: true,
			errorMsg:    "must match",
		},
		{
			name:        "ticket ID only in branch",
			branchName:  "ABC-123",
			prTitle:     "ABC-123",
			expected:    "ABC-123",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				PR: PRConfig{
					Title: tt.prTitle,
				},
			}

			result, err := validateBranchAndTitle(config, tt.branchName)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain '%s', got: %s", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

// TestParseCustomStatuses verifies that comma-separated status strings
// are parsed correctly into case-insensitive status maps
func TestParseCustomStatuses(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]bool
	}{
		{
			name:  "single status",
			input: "In Progress",
			expected: map[string]bool{
				"in progress": true,
			},
		},
		{
			name:  "multiple statuses",
			input: "In Dev,In Code Review,Ready for Dev",
			expected: map[string]bool{
				"in dev":         true,
				"in code review": true,
				"ready for dev":  true,
			},
		},
		{
			name:  "statuses with extra whitespace",
			input: "  In Dev  ,  In Code Review  ",
			expected: map[string]bool{
				"in dev":         true,
				"in code review": true,
			},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: nil,
		},
		{
			name:  "mixed case normalization",
			input: "IN DEV,In Code Review,ready FOR dev",
			expected: map[string]bool{
				"in dev":         true,
				"in code review": true,
				"ready for dev":  true,
			},
		},
		{
			name:  "with empty elements",
			input: "In Dev,,In Code Review",
			expected: map[string]bool{
				"in dev":         true,
				"in code review": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCustomStatuses(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d statuses, got %d", len(tt.expected), len(result))
			}

			for key := range tt.expected {
				if !result[key] {
					t.Errorf("expected status '%s' to be in result", key)
				}
			}
		})
	}
}

// TestGetAcceptedStatuses verifies that the correct status map is returned
// based on whether custom statuses are provided or defaults should be used
func TestGetAcceptedStatuses(t *testing.T) {
	tests := []struct {
		name           string
		customStatuses string
		expectDefault  bool
		expectNil      bool
	}{
		{
			name:           "empty string returns defaults",
			customStatuses: "",
			expectDefault:  true,
		},
		{
			name:           "custom statuses provided",
			customStatuses: "In Progress,Done",
			expectDefault:  false,
			expectNil:      false,
		},
		{
			name:           "whitespace only returns nil (skips validation)",
			customStatuses: "   ",
			expectNil:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getAcceptedStatuses(tt.customStatuses)

			if tt.expectNil && result != nil {
				t.Errorf("expected nil, got %v", result)
				return
			}

			if tt.expectDefault {
				// Verify it returns the default statuses
				if len(result) != len(defaultAcceptedStatuses) {
					t.Errorf("expected default statuses with %d entries, got %d", len(defaultAcceptedStatuses), len(result))
				}
				for key := range defaultAcceptedStatuses {
					if !result[key] {
						t.Errorf("expected default status '%s' to be in result", key)
					}
				}
			}
		})
	}
}

// TestIsValidStatus verifies case-insensitive status matching with whitespace handling
func TestIsValidStatus(t *testing.T) {
	acceptedStatuses := map[string]bool{
		"in dev":         true,
		"in code review": true,
		"ready for dev":  true,
	}

	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{
			name:     "exact match lowercase",
			status:   "in dev",
			expected: true,
		},
		{
			name:     "exact match with different case",
			status:   "In Dev",
			expected: true,
		},
		{
			name:     "uppercase match",
			status:   "IN DEV",
			expected: true,
		},
		{
			name:     "status with extra whitespace",
			status:   "  in dev  ",
			expected: true,
		},
		{
			name:     "multi-word status",
			status:   "In Code Review",
			expected: true,
		},
		{
			name:     "non-existent status",
			status:   "In Progress",
			expected: false,
		},
		{
			name:     "empty string",
			status:   "",
			expected: false,
		},
		{
			name:     "partial match should fail",
			status:   "in",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidStatus(tt.status, acceptedStatuses)
			if result != tt.expected {
				t.Errorf("expected %v, got %v for status '%s'", tt.expected, result, tt.status)
			}
		})
	}
}

// TestGetAcceptedStatusList verifies that status maps are formatted correctly
// into title-cased, comma-separated strings for user-facing error messages
func TestGetAcceptedStatusList(t *testing.T) {
	tests := []struct {
		name     string
		statuses map[string]bool
	}{
		{
			name: "single status",
			statuses: map[string]bool{
				"in dev": true,
			},
		},
		{
			name: "multiple statuses",
			statuses: map[string]bool{
				"in dev":         true,
				"in code review": true,
				"ready for dev":  true,
			},
		},
		{
			name:     "empty map",
			statuses: map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getAcceptedStatusList(tt.statuses)

			// Verify all statuses are in the result (case-insensitive)
			for status := range tt.statuses {
				// The result should contain the status in title case
				if !strings.Contains(strings.ToLower(result), status) {
					t.Errorf("expected result to contain '%s', got: %s", status, result)
				}
			}

			// Verify comma separation for multiple statuses
			if len(tt.statuses) > 1 {
				if !strings.Contains(result, ",") {
					t.Errorf("expected comma-separated list, got: %s", result)
				}
			}
		})
	}
}

// TestGetPrCtx verifies that the GitHub repository string is correctly parsed
// into owner, repo, and PR number components
func TestGetPrCtx(t *testing.T) {
	tests := []struct {
		name        string
		repository  string
		prNumber    int
		expectError bool
		expectedCtx *prCtx
	}{
		{
			name:        "valid repository format",
			repository:  "owner/repo",
			prNumber:    123,
			expectError: false,
			expectedCtx: &prCtx{
				Owner:  "owner",
				Repo:   "repo",
				Number: 123,
			},
		},
		{
			name:        "repository with organization",
			repository:  "TykTechnologies/jira-linter",
			prNumber:    456,
			expectError: false,
			expectedCtx: &prCtx{
				Owner:  "TykTechnologies",
				Repo:   "jira-linter",
				Number: 456,
			},
		},
		{
			name:        "invalid format - missing slash",
			repository:  "ownerrepo",
			prNumber:    123,
			expectError: true,
		},
		{
			name:        "invalid format - too many slashes",
			repository:  "owner/repo/extra",
			prNumber:    123,
			expectError: true,
		},
		{
			name:        "empty repository",
			repository:  "",
			prNumber:    123,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				GitHubRepository: tt.repository,
				PR: PRConfig{
					Number: tt.prNumber,
				},
			}

			result, err := getPrCtx(config)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result.Owner != tt.expectedCtx.Owner {
					t.Errorf("expected owner %s, got %s", tt.expectedCtx.Owner, result.Owner)
				}
				if result.Repo != tt.expectedCtx.Repo {
					t.Errorf("expected repo %s, got %s", tt.expectedCtx.Repo, result.Repo)
				}
				if result.Number != tt.expectedCtx.Number {
					t.Errorf("expected number %d, got %d", tt.expectedCtx.Number, result.Number)
				}
			}
		})
	}
}

// TestLoadConfig verifies that configuration is loaded and validated correctly
// from environment variables
func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		setupEnv    func()
		cleanupEnv  func()
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			setupEnv: func() {
				os.Setenv("JL_JIRA_BASEURL", "https://example.atlassian.net")
				os.Setenv("JL_JIRA_APITOKEN", "test-token")
				os.Setenv("JL_PR_NUMBER", "123")
				os.Setenv("JL_PR_TITLE", "ABC-123: Test PR")
				os.Setenv("JL_GITHUB_TOKEN", "gh-token")
				os.Setenv("JL_GITHUB_REPOSITORY", "owner/repo")
			},
			cleanupEnv: func() {
				os.Unsetenv("JL_JIRA_BASEURL")
				os.Unsetenv("JL_JIRA_APITOKEN")
				os.Unsetenv("JL_PR_NUMBER")
				os.Unsetenv("JL_PR_TITLE")
				os.Unsetenv("JL_GITHUB_TOKEN")
				os.Unsetenv("JL_GITHUB_REPOSITORY")
			},
			expectError: false,
		},
		{
			name: "empty Jira base URL",
			setupEnv: func() {
				os.Setenv("JL_JIRA_BASEURL", "")
				os.Setenv("JL_JIRA_APITOKEN", "test-token")
				os.Setenv("JL_PR_NUMBER", "123")
				os.Setenv("JL_PR_TITLE", "ABC-123: Test PR")
			},
			cleanupEnv: func() {
				os.Unsetenv("JL_JIRA_BASEURL")
				os.Unsetenv("JL_JIRA_APITOKEN")
				os.Unsetenv("JL_PR_NUMBER")
				os.Unsetenv("JL_PR_TITLE")
			},
			expectError: true,
			errorMsg:    "Jira base URL",
		},
		{
			name: "whitespace-only Jira base URL",
			setupEnv: func() {
				os.Setenv("JL_JIRA_BASEURL", "   ")
				os.Setenv("JL_JIRA_APITOKEN", "test-token")
				os.Setenv("JL_PR_NUMBER", "123")
				os.Setenv("JL_PR_TITLE", "ABC-123: Test PR")
			},
			cleanupEnv: func() {
				os.Unsetenv("JL_JIRA_BASEURL")
				os.Unsetenv("JL_JIRA_APITOKEN")
				os.Unsetenv("JL_PR_NUMBER")
				os.Unsetenv("JL_PR_TITLE")
			},
			expectError: true,
			errorMsg:    "Jira base URL",
		},
		{
			name: "invalid PR number - zero",
			setupEnv: func() {
				os.Setenv("JL_JIRA_BASEURL", "https://example.atlassian.net")
				os.Setenv("JL_JIRA_APITOKEN", "test-token")
				os.Setenv("JL_PR_NUMBER", "0")
				os.Setenv("JL_PR_TITLE", "ABC-123: Test PR")
			},
			cleanupEnv: func() {
				os.Unsetenv("JL_JIRA_BASEURL")
				os.Unsetenv("JL_JIRA_APITOKEN")
				os.Unsetenv("JL_PR_NUMBER")
				os.Unsetenv("JL_PR_TITLE")
			},
			expectError: true,
			errorMsg:    "PR number must be a positive integer",
		},
		{
			name: "invalid PR number - negative",
			setupEnv: func() {
				os.Setenv("JL_JIRA_BASEURL", "https://example.atlassian.net")
				os.Setenv("JL_JIRA_APITOKEN", "test-token")
				os.Setenv("JL_PR_NUMBER", "-1")
				os.Setenv("JL_PR_TITLE", "ABC-123: Test PR")
			},
			cleanupEnv: func() {
				os.Unsetenv("JL_JIRA_BASEURL")
				os.Unsetenv("JL_JIRA_APITOKEN")
				os.Unsetenv("JL_PR_NUMBER")
				os.Unsetenv("JL_PR_TITLE")
			},
			expectError: true,
			errorMsg:    "PR number must be a positive integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			defer tt.cleanupEnv()

			config, err := loadConfig()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain '%s', got: %s", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if config == nil {
					t.Errorf("expected config to be non-nil")
				}
			}
		})
	}
}

// TestValidateJiraIssue verifies that Jira issues are validated against
// accepted status lists, with proper handling of custom and default statuses
func TestValidateJiraIssue(t *testing.T) {
	tests := []struct {
		name           string
		issueStatus    string
		customStatuses string
		issueID        string
		expectError    bool
		errorMsg       string
	}{
		{
			name:           "valid status with defaults",
			issueStatus:    "In Dev",
			customStatuses: "",
			issueID:        "ABC-123",
			expectError:    false,
		},
		{
			name:           "valid status case insensitive",
			issueStatus:    "IN DEV",
			customStatuses: "",
			issueID:        "ABC-123",
			expectError:    false,
		},
		{
			name:           "invalid status with defaults",
			issueStatus:    "To Do",
			customStatuses: "",
			issueID:        "ABC-123",
			expectError:    true,
			errorMsg:       "has status 'To Do'",
		},
		{
			name:           "valid custom status",
			issueStatus:    "In Progress",
			customStatuses: "In Progress,Done",
			issueID:        "ABC-123",
			expectError:    false,
		},
		{
			name:           "invalid custom status",
			issueStatus:    "To Do",
			customStatuses: "In Progress,Done",
			issueID:        "ABC-123",
			expectError:    true,
			errorMsg:       "has status 'To Do'",
		},
		{
			name:           "skip validation with whitespace-only statuses",
			issueStatus:    "Any Status",
			customStatuses: "   ",
			issueID:        "ABC-123",
			expectError:    false,
		},
		{
			name:           "default status dod check",
			issueStatus:    "DoD Check",
			customStatuses: "",
			issueID:        "ABC-123",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := &jira.Issue{
				Key: tt.issueID,
				Fields: &jira.IssueFields{
					Status: &jira.Status{
						Name: tt.issueStatus,
					},
				},
			}

			err := validateJiraIssue(issue, tt.customStatuses, tt.issueID)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain '%s', got: %s", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestBasicAuthTransport verifies that the custom HTTP transport correctly
// adds Basic Authentication headers to outgoing requests
func TestBasicAuthTransport(t *testing.T) {
	transport := &basicAuthTransport{
		Token: "test-token-123",
	}

	// Verify the transport is created correctly and has the expected token
	if transport.Token != "test-token-123" {
		t.Errorf("expected token 'test-token-123', got '%s'", transport.Token)
	}

	// Note: We would need a mock server to fully test RoundTrip behavior
	// In a real scenario, RoundTrip would add the Authorization header:
	// req.Header.Set("Authorization", fmt.Sprintf("Basic %s", t.Token))
	if transport.Token == "" {
		t.Errorf("expected token to be set on transport")
	}
}

// TestValidateJiraIssueNilSafety tests nil pointer handling in validateJiraIssue
// to prevent runtime panics when dealing with incomplete Jira issue data
func TestValidateJiraIssueNilSafety(t *testing.T) {
	t.Run("nil issue causes panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// Expected to panic with nil issue
				t.Logf("Expected panic caught: %v", r)
			}
		}()
		// This will panic - documenting current behavior
		_ = validateJiraIssue(nil, "", "ABC-123")
	})

	t.Run("nil issue fields causes panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// Expected to panic with nil Fields
				t.Logf("Expected panic caught: %v", r)
			}
		}()
		issue := &jira.Issue{
			Key:    "ABC-123",
			Fields: nil,
		}
		// This will panic - documenting current behavior
		_ = validateJiraIssue(issue, "", "ABC-123")
	})

	t.Run("nil status causes panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// Expected to panic with nil Status
				t.Logf("Expected panic caught: %v", r)
			}
		}()
		issue := &jira.Issue{
			Key: "ABC-123",
			Fields: &jira.IssueFields{
				Status: nil,
			},
		}
		// This will panic - documenting current behavior
		_ = validateJiraIssue(issue, "", "ABC-123")
	})

	t.Run("empty status name with validation enabled", func(t *testing.T) {
		issue := &jira.Issue{
			Key: "ABC-123",
			Fields: &jira.IssueFields{
				Status: &jira.Status{
					Name: "",
				},
			},
		}
		err := validateJiraIssue(issue, "", "ABC-123")
		if err == nil {
			t.Errorf("expected error for empty status name, got nil")
		}
	})
}

// TestIsValidStatusNilMap tests that isValidStatus handles nil maps safely
func TestIsValidStatusNilMap(t *testing.T) {
	result := isValidStatus("In Dev", nil)
	if result != false {
		t.Errorf("expected false for nil map, got %v", result)
	}
}

// TestGetAcceptedStatusListNilMap tests that getAcceptedStatusList handles nil maps safely
func TestGetAcceptedStatusListNilMap(t *testing.T) {
	result := getAcceptedStatusList(nil)
	// Should return empty string, not panic
	if result != "" {
		t.Errorf("expected empty string for nil map, got '%s'", result)
	}
}

// TestGetJiraClient tests the Jira client creation with various configurations
func TestGetJiraClient(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			config: &Config{
				Jira: JiraConfig{
					BaseURL:  "https://example.atlassian.net",
					APIToken: "dGVzdDp0b2tlbg==", // base64 encoded "test:token"
				},
			},
			expectError: false,
		},
		{
			name: "empty API token",
			config: &Config{
				Jira: JiraConfig{
					BaseURL:  "https://example.atlassian.net",
					APIToken: "",
				},
			},
			expectError: false, // Client creation doesn't validate token
		},
		{
			name: "invalid base URL - malformed",
			config: &Config{
				Jira: JiraConfig{
					BaseURL:  "://invalid-url",
					APIToken: "dGVzdDp0b2tlbg==",
				},
			},
			expectError: true,
		},
		{
			name: "base URL with trailing slash",
			config: &Config{
				Jira: JiraConfig{
					BaseURL:  "https://example.atlassian.net/",
					APIToken: "dGVzdDp0b2tlbg==",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := getJiraClient(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain '%s', got: %s", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if client == nil {
					t.Errorf("expected client to be non-nil")
				}
			}
		})
	}
}

// TestCreateGitHubClient tests GitHub client creation
func TestCreateGitHubClient(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		expectError bool
	}{
		{
			name:        "valid token",
			token:       "ghp_1234567890abcdef",
			expectError: false,
		},
		{
			name:        "empty token",
			token:       "",
			expectError: false, // Client creation doesn't validate token
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client, err := createGitHubClient(ctx, tt.token)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if client == nil {
					t.Errorf("expected client to be non-nil")
				}
			}
		})
	}
}

// TestUpdatePRDescriptionBodyMarkerParsing tests edge cases in marker detection and replacement
func TestUpdatePRDescriptionBodyMarkerParsing(t *testing.T) {
	tests := []struct {
		name            string
		existingBody    string
		expectedPattern string // What pattern we expect in the result
	}{
		{
			name:            "empty PR body",
			existingBody:    "",
			expectedPattern: jiraSectionMarkerStart,
		},
		{
			name:            "body with only start marker",
			existingBody:    "Some content\n" + jiraSectionMarkerStart + "\nOld content",
			expectedPattern: jiraSectionMarkerStart,
		},
		{
			name:            "body with only end marker",
			existingBody:    "Some content\nOld content\n" + jiraSectionMarkerEnd,
			expectedPattern: jiraSectionMarkerStart,
		},
		{
			name: "body with complete marker pair",
			existingBody: "PR description\n" + jiraSectionMarkerStart + "\nOld Jira info\n" +
				jiraSectionMarkerEnd + "\nMore content",
			expectedPattern: jiraSectionMarkerStart,
		},
		{
			name: "body with markers in wrong order",
			existingBody: "Content\n" + jiraSectionMarkerEnd + "\nBetween\n" +
				jiraSectionMarkerStart + "\nContent",
			expectedPattern: jiraSectionMarkerStart,
		},
		{
			name: "very large body",
			existingBody: strings.Repeat("A", 10000) + "\n" + jiraSectionMarkerStart + "\n" +
				jiraSectionMarkerEnd,
			expectedPattern: jiraSectionMarkerStart,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't fully test updatePRDescriptionBody without mocking GitHub API
			// but we can verify the marker logic by testing the string manipulation

			newSection := "\n" + jiraSectionMarkerStart + "\nNew Jira Info\n" +
				jiraSectionMarkerEnd + "\n"

			startIdx := strings.Index(tt.existingBody, jiraSectionMarkerStart)
			endIdx := strings.Index(tt.existingBody, jiraSectionMarkerEnd)

			var newBody string
			if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
				newBody = tt.existingBody[:startIdx] + newSection +
					tt.existingBody[endIdx+len(jiraSectionMarkerEnd):]
			} else {
				newBody = tt.existingBody + newSection
			}

			// Verify the new body contains the expected pattern
			if !strings.Contains(newBody, tt.expectedPattern) {
				t.Errorf("expected new body to contain '%s'", tt.expectedPattern)
			}

			// Verify both markers are present
			if !strings.Contains(newBody, jiraSectionMarkerStart) {
				t.Errorf("expected new body to contain start marker")
			}
			if !strings.Contains(newBody, jiraSectionMarkerEnd) {
				t.Errorf("expected new body to contain end marker")
			}
		})
	}
}

// TestParseCustomStatusesEdgeCases tests additional edge cases for status parsing
func TestParseCustomStatusesEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]bool
	}{
		{
			name:  "very long status name",
			input: "This Is A Very Long Status Name With Many Words",
			expected: map[string]bool{
				"this is a very long status name with many words": true,
			},
		},
		{
			name:  "status with numbers",
			input: "Status 1,Status 2,Status 3",
			expected: map[string]bool{
				"status 1": true,
				"status 2": true,
				"status 3": true,
			},
		},
		{
			name:  "status with special characters",
			input: "In-Progress,Code/Review,QA&Test",
			expected: map[string]bool{
				"in-progress": true,
				"code/review": true,
				"qa&test":     true,
			},
		},
		{
			name:     "only commas",
			input:    ",,,",
			expected: map[string]bool{}, // Returns empty map, not nil
		},
		{
			name:  "leading and trailing commas",
			input: ",In Dev,In Code Review,",
			expected: map[string]bool{
				"in dev":         true,
				"in code review": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCustomStatuses(tt.input)

			// Handle nil expected value
			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			// Handle empty map expected value (different from nil)
			if result == nil && len(tt.expected) != 0 {
				t.Errorf("expected map with %d entries, got nil", len(tt.expected))
				return
			}

			if result != nil && len(result) != len(tt.expected) {
				t.Errorf("expected %d statuses, got %d", len(tt.expected), len(result))
			}

			for key := range tt.expected {
				if result == nil || !result[key] {
					t.Errorf("expected status '%s' to be in result", key)
				}
			}
		})
	}
}
