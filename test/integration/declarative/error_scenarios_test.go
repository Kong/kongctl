//go:build integration

package declarative_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/declarative"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestExecutorAPIErrors(t *testing.T) {
	testCases := []struct {
		name               string
		command            string
		mockSetup          func(*MockPortalAPI)
		expectedError      string
		expectFailure      bool
		expectSummaryError bool // true when failure occurs during execution stage
	}{
		{
			name:    "Network Error during List",
			command: "apply",
			mockSetup: func(mockAPI *MockPortalAPI) {
				mockAPI.On("ListPortals", mock.Anything, mock.Anything).Return(
					nil, fmt.Errorf("connection refused"))
			},
			expectedError:      "connection refused",
			expectFailure:      true,
			expectSummaryError: false, // fails during plan generation (List)
		},
		{
			name:    "Context Timeout during List",
			command: "apply",
			mockSetup: func(mockAPI *MockPortalAPI) {
				mockAPI.On("ListPortals", mock.Anything, mock.Anything).Return(
					nil, context.DeadlineExceeded)
			},
			expectedError:      "context deadline exceeded",
			expectFailure:      true,
			expectSummaryError: false, // fails during plan generation (List)
		},
		{
			name:    "API Creation Failure",
			command: "apply",
			mockSetup: func(mockAPI *MockPortalAPI) {
				// Successful list
				mockAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
					StatusCode: 200,
					ListPortalsResponse: &kkComps.ListPortalsResponse{
						Data: []kkComps.ListPortalsResponsePortal{},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 0},
						},
					},
				}, nil)
				// Failed creation
				mockAPI.On("CreatePortal", mock.Anything, mock.Anything).Return(
					nil, fmt.Errorf("validation failed: name already exists"))
			},
			expectedError:      "validation failed",
			expectFailure:      true,
			expectSummaryError: true, // fails during execution (Create)
		},
		{
			name:    "Rate Limit Error during Creation",
			command: "apply",
			mockSetup: func(mockAPI *MockPortalAPI) {
				// Successful list
				mockAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
					StatusCode: 200,
					ListPortalsResponse: &kkComps.ListPortalsResponse{
						Data: []kkComps.ListPortalsResponsePortal{},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 0},
						},
					},
				}, nil)
				// Rate limited creation
				mockAPI.On("CreatePortal", mock.Anything, mock.Anything).Return(
					nil, fmt.Errorf("rate limit exceeded"))
			},
			expectedError:      "rate limit exceeded",
			expectFailure:      true,
			expectSummaryError: true, // fails during execution (Create)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test configuration
			configDir := t.TempDir()
			configFile := filepath.Join(configDir, "config.yaml")

			config := `
portals:
  - ref: error-portal
    name: "Error Test Portal"
    description: "Portal for API error testing"
`
			require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

			// Set up test context with mocks
			ctx := SetupTestContext(t)

			// Get the mock SDK and set up expectations
			sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
			konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
			mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
			mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)

			// Set up mock API behavior for the test case
			tc.mockSetup(mockPortalAPI)

			// Set up auth and API mocks
			setupEmptyAPIMocks(mockSDK)

			// Create command
			cmd, err := declarative.NewDeclarativeCmd(verbs.VerbValue(tc.command))
			require.NoError(t, err)

			// Set context
			cmd.SetContext(ctx)

			// Capture output
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)

			// Run command
			cmd.SetArgs([]string{"-f", configFile, "--auto-approve"})
			err = cmd.Execute()

			// Verify error handling
			if tc.expectFailure {
				require.Error(t, err)
				if tc.expectSummaryError {
					// Execution-stage failure: summary error with count, details in output
					assert.Regexp(t, `^execution completed with \d+ errors`, err.Error())
					assert.Contains(t, output.String(), tc.expectedError)
				} else {
					// Plan-generation failure: underlying error is returned directly
					assert.Contains(t, err.Error(), tc.expectedError)
				}
			} else {
				require.NoError(t, err)
			}

			// Verify mocks were called as expected
			mockPortalAPI.AssertExpectations(t)
		})
	}
}

func TestExecutorPartialFailure(t *testing.T) {
	// Create test configuration with multiple portals
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "multi-portal.yaml")

	config := `
portals:
  - ref: success-portal
    name: "Success Portal"
    description: "This portal should be created successfully"
  - ref: failure-portal
    name: "Failure Portal"
    description: "This portal creation should fail"
  - ref: another-success-portal
    name: "Another Success Portal"
    description: "This should also succeed"
`
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

	// Set up test context with mocks
	ctx := SetupTestContext(t)

	// Get the mock SDK and set up expectations
	sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)

	// Mock empty current state
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		StatusCode: 200,
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.ListPortalsResponsePortal{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: 0},
			},
		},
	}, nil)

	// Mock successful creation for first portal
	mockPortalAPI.On("CreatePortal", mock.Anything, mock.MatchedBy(func(portal kkComps.CreatePortal) bool {
		return portal.Name == "Success Portal"
	})).Return(&kkOps.CreatePortalResponse{
		StatusCode: 201,
		PortalResponse: &kkComps.PortalResponse{
			ID:          "success-portal-id",
			Name:        "Success Portal",
			Description: stringPtr("This portal should be created successfully"),
			DisplayName: "",
			Labels:      make(map[string]string),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}, nil)

	// Mock failure for second portal
	mockPortalAPI.On("CreatePortal", mock.Anything, mock.MatchedBy(func(portal kkComps.CreatePortal) bool {
		return portal.Name == "Failure Portal"
	})).Return(&kkOps.CreatePortalResponse{
		StatusCode: 422,
	}, nil)

	// Mock successful creation for third portal
	mockPortalAPI.On("CreatePortal", mock.Anything, mock.MatchedBy(func(portal kkComps.CreatePortal) bool {
		return portal.Name == "Another Success Portal"
	})).Return(&kkOps.CreatePortalResponse{
		StatusCode: 201,
		PortalResponse: &kkComps.PortalResponse{
			ID:          "another-success-portal-id",
			Name:        "Another Success Portal",
			Description: stringPtr("This should also succeed"),
			DisplayName: "",
			Labels:      make(map[string]string),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}, nil)

	// Set up auth and API mocks
	setupEmptyAPIMocks(mockSDK)

	// Create apply command
	cmd, err := declarative.NewDeclarativeCmd(verbs.Apply)
	require.NoError(t, err)

	// Set context
	cmd.SetContext(ctx)

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Run apply with auto-approve
	cmd.SetArgs([]string{"-f", configFile, "--auto-approve"})
	err = cmd.Execute()

	// The command should complete but report partial failure
	// In this case, we expect the command to complete with some errors
	outputStr := output.String()

	// Check that successful operations are reported
	assert.Contains(t, outputStr, "Creating portal: success-portal")
	assert.Contains(t, outputStr, "Creating portal: another-success-portal")

	// Partial failures return a summary error and include details in output
	require.Error(t, err)
	assert.Regexp(t, `^execution completed with \d+ errors`, err.Error())
	assert.Contains(t, outputStr, "failure-portal")

	// Verify mocks were called as expected
	mockPortalAPI.AssertExpectations(t)
}

func TestProtectionViolations(t *testing.T) {
	testCases := []struct {
		name          string
		command       string
		config        string
		expectedError string
	}{
		{
			name:    "Sync - Protected Resource Deletion",
			command: "sync",
			config: `
portals: []  # Empty - would delete all managed resources including protected ones
`,
			expectedError: "protected",
		},
		{
			name:    "Sync - Protected Resource Modification",
			command: "sync",
			config: `
portals:
  - ref: protected-portal
    name: "Modified Protected Portal"
    description: "Attempt to modify protected portal"
`,
			expectedError: "protected",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test configuration
			configDir := t.TempDir()
			configFile := filepath.Join(configDir, "protected.yaml")
			require.NoError(t, os.WriteFile(configFile, []byte(tc.config), 0o600))

			// Set up test context with mocks
			ctx := SetupTestContext(t)

			// Get the mock SDK and set up expectations
			sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
			konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
			mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
			mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)

			// Mock current state with a protected portal
			protectedPortal := CreateManagedPortal("Protected Portal", "protected-id", "Original description")
			protectedPortal.Labels[labels.ProtectedKey] = "true"

			mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
				StatusCode: 200,
				ListPortalsResponse: &kkComps.ListPortalsResponse{
					Data: []kkComps.ListPortalsResponsePortal{protectedPortal},
					Meta: kkComps.PaginatedMeta{
						Page: kkComps.PageMeta{Total: 1},
					},
				},
			}, nil)

			// Set up auth and API mocks
			setupEmptyAPIMocks(mockSDK)

			// Create command
			cmd, err := declarative.NewDeclarativeCmd(verbs.VerbValue(tc.command))
			require.NoError(t, err)

			// Set context
			cmd.SetContext(ctx)

			// Capture output
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)

			// Run command - should fail due to protection violation
			cmd.SetArgs([]string{"-f", configFile, "--auto-approve"})
			err = cmd.Execute()

			// Verify protection violation is detected
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)

			// Verify no API modification calls were made (fail-fast behavior)
			mockPortalAPI.AssertExpectations(t)
		})
	}
}

func TestNetworkFailures(t *testing.T) {
	testCases := []struct {
		name          string
		mockSetup     func(*MockPortalAPI)
		expectedError string
	}{
		{
			name: "Context Deadline Exceeded",
			mockSetup: func(mockAPI *MockPortalAPI) {
				mockAPI.On("ListPortals", mock.Anything, mock.Anything).Return(
					nil, context.DeadlineExceeded)
			},
			expectedError: "context deadline exceeded",
		},
		{
			name: "Connection Refused",
			mockSetup: func(mockAPI *MockPortalAPI) {
				connErr := &net.OpError{
					Op:  "dial",
					Err: fmt.Errorf("connection refused"),
				}
				mockAPI.On("ListPortals", mock.Anything, mock.Anything).Return(nil, connErr)
			},
			expectedError: "connection refused",
		},
		{
			name: "DNS Resolution Failure",
			mockSetup: func(mockAPI *MockPortalAPI) {
				dnsErr := &net.DNSError{
					Err:    "no such host",
					Name:   "api.konghq.com",
					Server: "8.8.8.8:53",
				}
				mockAPI.On("ListPortals", mock.Anything, mock.Anything).Return(nil, dnsErr)
			},
			expectedError: "no such host",
		},
		{
			name: "Intermittent Network Error",
			mockSetup: func(mockAPI *MockPortalAPI) {
				netErr := fmt.Errorf("network is unreachable")
				mockAPI.On("ListPortals", mock.Anything, mock.Anything).Return(nil, netErr)
			},
			expectedError: "network is unreachable",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test configuration
			configDir := t.TempDir()
			configFile := filepath.Join(configDir, "network-test.yaml")

			config := `
portals:
  - ref: network-test-portal
    name: "Network Test Portal"
    description: "Portal for network failure testing"
`
			require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

			// Set up test context with mocks
			ctx := SetupTestContext(t)

			// Get the mock SDK and set up expectations
			sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
			konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
			mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
			mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)

			// Set up network failure mock
			tc.mockSetup(mockPortalAPI)

			// Set up auth and API mocks (though they may not be called due to early failure)
			setupEmptyAPIMocks(mockSDK)

			// Create apply command
			cmd, err := declarative.NewDeclarativeCmd("apply")
			require.NoError(t, err)

			// Set context
			cmd.SetContext(ctx)

			// Capture output
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)

			// Run command - should fail due to network error
			cmd.SetArgs([]string{"-f", configFile, "--auto-approve"})
			err = cmd.Execute()

			// Verify network failure is handled gracefully
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)

			// Verify mocks were called as expected
			mockPortalAPI.AssertExpectations(t)
		})
	}
}

func TestInvalidConfigurations(t *testing.T) {
	testCases := []struct {
		name          string
		config        string
		expectedError string
	}{
		{
			name: "Malformed YAML Syntax",
			config: `
portals:
  - ref: malformed-portal
    name: "Malformed Portal"
    invalid_yaml: [unclosed bracket
`,
			expectedError: "failed to parse YAML",
		},
		{
			name: "Missing Required Field - Ref",
			config: `
portals:
  - name: "Portal Without Ref"
    description: "Missing ref field"
`,
			expectedError: "ref cannot be empty",
		},
		{
			name: "Duplicate Resource Refs",
			config: `
portals:
  - ref: duplicate-portal
    name: "Portal 1"
    description: "First portal"
  - ref: duplicate-portal
    name: "Portal 2"
    description: "Second portal with same ref"
`,
			expectedError: "duplicate",
		},
		{
			name: "Invalid Cross-Resource Reference",
			config: `
api_publications:
  - ref: invalid-pub
    api: test-api
    portal_id: nonexistent-portal
    visibility: public

apis:
  - ref: test-api
    name: "Test API"
    description: "Test API"
    version: "1.0.0"
`,
			expectedError: "unknown portal",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test configuration file
			configDir := t.TempDir()
			configFile := filepath.Join(configDir, "invalid.yaml")
			require.NoError(t, os.WriteFile(configFile, []byte(tc.config), 0o600))

			// Set up test context
			ctx := SetupTestContext(t)

			// Create apply command
			cmd, err := declarative.NewDeclarativeCmd(verbs.Apply)
			require.NoError(t, err)

			// Set context
			cmd.SetContext(ctx)

			// Capture output
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)

			// Run command - should fail due to invalid configuration
			cmd.SetArgs([]string{"-f", configFile, "--auto-approve"})
			err = cmd.Execute()

			// Verify configuration validation catches the error
			require.Error(t, err)

			// Check that the error message contains expected information
			errorMsg := strings.ToLower(err.Error())
			expectedMsg := strings.ToLower(tc.expectedError)
			assert.Contains(t, errorMsg, expectedMsg,
				"Expected error to contain '%s', but got: %s", tc.expectedError, err.Error())
		})
	}
}
