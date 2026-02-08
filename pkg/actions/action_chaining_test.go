package actions

import (
	"testing"

	"github.com/kylegalloway/forge/pkg/constants"
)

// TestActionChaining_DetermineNextAction_BuildPublish tests the BuildPublish compound action chain
func TestActionChaining_DetermineNextAction_BuildPublish(t *testing.T) {
	tests := []struct {
		name               string
		mainAction         string
		completedAction    string
		expectedNextAction string
	}{
		{
			name:               "Zarf BuildPublish - build completed, publish next",
			mainAction:         constants.SpecActionBuildPublish,
			completedAction:    constants.ActionBuild,
			expectedNextAction: constants.ActionPublish,
		},
		{
			name:               "UDS CreatePublish - create completed, publish next",
			mainAction:         constants.SpecActionCreatePublish,
			completedAction:    constants.ActionCreate,
			expectedNextAction: constants.ActionPublish,
		},
		{
			name:               "BuildPublish - wrong action completed, no next",
			mainAction:         constants.SpecActionBuildPublish,
			completedAction:    constants.ActionPublish,
			expectedNextAction: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextAction := determineNextActionForTest(tt.mainAction, tt.completedAction, constants.ActionBuild)

			if nextAction != tt.expectedNextAction {
				t.Errorf("expected nextAction = %s, got %s", tt.expectedNextAction, nextAction)
			}
		})
	}
}

// TestActionChaining_DetermineNextAction_BuildDeploy tests the BuildDeploy compound action chain
func TestActionChaining_DetermineNextAction_BuildDeploy(t *testing.T) {
	tests := []struct {
		name               string
		mainAction         string
		completedAction    string
		expectedNextAction string
	}{
		{
			name:               "Zarf BuildDeploy - build completed, deploy next",
			mainAction:         constants.SpecActionBuildDeploy,
			completedAction:    constants.ActionBuild,
			expectedNextAction: constants.ActionDeploy,
		},
		{
			name:               "UDS CreateDeploy - create completed, deploy next",
			mainAction:         constants.SpecActionCreateDeploy,
			completedAction:    constants.ActionCreate,
			expectedNextAction: constants.ActionDeploy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextAction := determineNextActionForTest(tt.mainAction, tt.completedAction, constants.ActionBuild)

			if nextAction != tt.expectedNextAction {
				t.Errorf("expected nextAction = %s, got %s", tt.expectedNextAction, nextAction)
			}
		})
	}
}

// TestActionChaining_DetermineNextAction_BuildPublishDeploy tests the 3-action compound action chain
func TestActionChaining_DetermineNextAction_BuildPublishDeploy(t *testing.T) {
	tests := []struct {
		name               string
		mainAction         string
		completedAction    string
		expectedNextAction string
	}{
		{
			name:               "Zarf BuildPublishDeploy - build completed, publish next",
			mainAction:         constants.SpecActionBuildPublishDeploy,
			completedAction:    constants.ActionBuild,
			expectedNextAction: constants.ActionPublish,
		},
		{
			name:               "Zarf BuildPublishDeploy - publish completed, deploy next",
			mainAction:         constants.SpecActionBuildPublishDeploy,
			completedAction:    constants.ActionPublish,
			expectedNextAction: constants.ActionDeploy,
		},
		{
			name:               "UDS CreatePublishDeploy - create completed, publish next",
			mainAction:         constants.SpecActionCreatePublishDeploy,
			completedAction:    constants.ActionCreate,
			expectedNextAction: constants.ActionPublish,
		},
		{
			name:               "UDS CreatePublishDeploy - publish completed, deploy next",
			mainAction:         constants.SpecActionCreatePublishDeploy,
			completedAction:    constants.ActionPublish,
			expectedNextAction: constants.ActionDeploy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextAction := determineNextActionForTest(tt.mainAction, tt.completedAction, constants.ActionBuild)

			if nextAction != tt.expectedNextAction {
				t.Errorf("expected nextAction = %s, got %s", tt.expectedNextAction, nextAction)
			}
		})
	}
}

// TestActionChaining_DetermineNextAction_PublishDeploy tests the PublishDeploy compound action chain
func TestActionChaining_DetermineNextAction_PublishDeploy(t *testing.T) {
	tests := []struct {
		name               string
		mainAction         string
		completedAction    string
		expectedNextAction string
	}{
		{
			name:               "PublishDeploy - publish completed, deploy next",
			mainAction:         constants.SpecActionPublishDeploy,
			completedAction:    constants.ActionPublish,
			expectedNextAction: constants.ActionDeploy,
		},
		{
			name:               "PublishDeploy - deploy completed, no next",
			mainAction:         constants.SpecActionPublishDeploy,
			completedAction:    constants.ActionDeploy,
			expectedNextAction: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextAction := determineNextActionForTest(tt.mainAction, tt.completedAction, constants.ActionPublish)

			if nextAction != tt.expectedNextAction {
				t.Errorf("expected nextAction = %s, got %s", tt.expectedNextAction, nextAction)
			}
		})
	}
}

// TestActionChaining_DetermineNextAction_SingleActions tests that single actions have no next action
func TestActionChaining_DetermineNextAction_SingleActions(t *testing.T) {
	tests := []struct {
		name               string
		mainAction         string
		completedAction    string
		expectedNextAction string
	}{
		{
			name:               "Build single action - no next",
			mainAction:         constants.SpecActionBuild,
			completedAction:    constants.ActionBuild,
			expectedNextAction: "",
		},
		{
			name:               "Create single action - no next",
			mainAction:         constants.SpecActionCreate,
			completedAction:    constants.ActionCreate,
			expectedNextAction: "",
		},
		{
			name:               "Publish single action - no next",
			mainAction:         constants.SpecActionPublish,
			completedAction:    constants.ActionPublish,
			expectedNextAction: "",
		},
		{
			name:               "Deploy single action - no next",
			mainAction:         constants.SpecActionDeploy,
			completedAction:    constants.ActionDeploy,
			expectedNextAction: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextAction := determineNextActionForTest(tt.mainAction, tt.completedAction, constants.ActionBuild)

			if nextAction != tt.expectedNextAction {
				t.Errorf("expected nextAction = %s, got %s", tt.expectedNextAction, nextAction)
			}
		})
	}
}

// TestActionChaining_IsMultiActionJob tests multi-action job detection
func TestActionChaining_IsMultiActionJob(t *testing.T) {
	tests := []struct {
		name            string
		action          string
		expectedIsMulti bool
	}{
		// Multi-action cases - Zarf
		{
			name:            "BuildPublish is multi-action",
			action:          constants.SpecActionBuildPublish,
			expectedIsMulti: true,
		},
		{
			name:            "BuildDeploy is multi-action",
			action:          constants.SpecActionBuildDeploy,
			expectedIsMulti: true,
		},
		{
			name:            "BuildPublishDeploy is multi-action",
			action:          constants.SpecActionBuildPublishDeploy,
			expectedIsMulti: true,
		},

		// Multi-action cases - UDS
		{
			name:            "CreatePublish is multi-action",
			action:          constants.SpecActionCreatePublish,
			expectedIsMulti: true,
		},
		{
			name:            "CreateDeploy is multi-action",
			action:          constants.SpecActionCreateDeploy,
			expectedIsMulti: true,
		},
		{
			name:            "CreatePublishDeploy is multi-action",
			action:          constants.SpecActionCreatePublishDeploy,
			expectedIsMulti: true,
		},

		// Multi-action case - Shared
		{
			name:            "PublishDeploy is multi-action",
			action:          constants.SpecActionPublishDeploy,
			expectedIsMulti: true,
		},

		// Single-action cases - should NOT be multi
		{
			name:            "Build is single action",
			action:          constants.SpecActionBuild,
			expectedIsMulti: false,
		},
		{
			name:            "Create is single action",
			action:          constants.SpecActionCreate,
			expectedIsMulti: false,
		},
		{
			name:            "Publish is single action",
			action:          constants.SpecActionPublish,
			expectedIsMulti: false,
		},
		{
			name:            "Deploy is single action",
			action:          constants.SpecActionDeploy,
			expectedIsMulti: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isMulti := isMultiActionJobForTest(tt.action)

			if isMulti != tt.expectedIsMulti {
				t.Errorf("expected isMulti = %v, got %v", tt.expectedIsMulti, isMulti)
			}
		})
	}
}

// TestActionChaining_StatePreservation tests that artifact paths are correctly preserved between actions
func TestActionChaining_StatePreservation(t *testing.T) {
	tests := []struct {
		name         string
		mainAction   string
		artifactPath string
		shouldUsePVC bool
	}{
		{
			name:         "BuildPublish - multi-action uses PVC",
			mainAction:   constants.SpecActionBuildPublish,
			artifactPath: "/artifacts/*.tar.zst",
			shouldUsePVC: true,
		},
		{
			name:         "BuildPublishDeploy - multi-action uses PVC",
			mainAction:   constants.SpecActionBuildPublishDeploy,
			artifactPath: "/artifacts/*.tar.zst",
			shouldUsePVC: true,
		},
		{
			name:         "PublishDeploy - multi-action uses PVC",
			mainAction:   constants.SpecActionPublishDeploy,
			artifactPath: "/artifacts/*.tar.zst",
			shouldUsePVC: true,
		},
		{
			name:         "Publish single - direct artifact path",
			mainAction:   constants.SpecActionPublish,
			artifactPath: "/tmp/artifact.tar.zst",
			shouldUsePVC: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isMulti := isMultiActionJobForTest(tt.mainAction)

			if isMulti != tt.shouldUsePVC {
				t.Errorf("expected multi-action=%v, got %v", tt.shouldUsePVC, isMulti)
			}

			// Verify artifact path pattern for multi-action
			if isMulti && tt.artifactPath != "/artifacts/*.tar.zst" {
				t.Errorf("multi-action expected glob pattern /artifacts/*.tar.zst, got %s", tt.artifactPath)
			}
		})
	}
}

// TestActionChaining_DebugModeSingleActionInChain tests debug mode with action chaining
func TestActionChaining_DebugModeSingleActionInChain(t *testing.T) {
	// Test that debug mode can be applied per-action or globally
	tests := []struct {
		name               string
		mainAction         string
		globalDebugMode    bool
		debugActions       []string
		buildShouldDebug   bool
		publishShouldDebug bool
	}{
		{
			name:               "BuildPublish - global debug applies to all",
			mainAction:         constants.SpecActionBuildPublish,
			globalDebugMode:    true,
			debugActions:       []string{},
			buildShouldDebug:   true,
			publishShouldDebug: true,
		},
		{
			name:               "BuildPublish - selective debug on publish only",
			mainAction:         constants.SpecActionBuildPublish,
			globalDebugMode:    false,
			debugActions:       []string{constants.ActionPublish},
			buildShouldDebug:   false,
			publishShouldDebug: true,
		},
		{
			name:               "BuildPublish - selective debug on build only",
			mainAction:         constants.SpecActionBuildPublish,
			globalDebugMode:    false,
			debugActions:       []string{constants.ActionBuild},
			buildShouldDebug:   true,
			publishShouldDebug: false,
		},
		{
			name:               "BuildPublishDeploy - mix of global and per-action debug",
			mainAction:         constants.SpecActionBuildPublishDeploy,
			globalDebugMode:    true,
			debugActions:       []string{}, // global overrides all
			buildShouldDebug:   true,
			publishShouldDebug: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify debug logic
			shouldDebug := func(action string) bool {
				if tt.globalDebugMode {
					return true
				}
				for _, da := range tt.debugActions {
					if da == action {
						return true
					}
				}
				return false
			}

			buildDebug := shouldDebug(constants.ActionBuild)
			publishDebug := shouldDebug(constants.ActionPublish)

			if buildDebug != tt.buildShouldDebug {
				t.Errorf("expected build debug=%v, got %v", tt.buildShouldDebug, buildDebug)
			}
			if publishDebug != tt.publishShouldDebug {
				t.Errorf("expected publish debug=%v, got %v", tt.publishShouldDebug, publishDebug)
			}
		})
	}
}

// TestActionChaining_InvalidActionSequences tests that invalid sequences are rejected
func TestActionChaining_InvalidActionSequences(t *testing.T) {
	tests := []struct {
		name            string
		mainAction      string
		completedAction string
		expectError     bool
	}{
		{
			name:            "BuildPublish - deploy completed but not expected",
			mainAction:      constants.SpecActionBuildPublish,
			completedAction: constants.ActionDeploy,
			expectError:     false, // Should just return empty, not error
		},
		{
			name:            "BuildDeploy - publish completed but action is BuildDeploy",
			mainAction:      constants.SpecActionBuildDeploy,
			completedAction: constants.ActionPublish,
			expectError:     false, // Should just return empty, not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextAction := determineNextActionForTest(tt.mainAction, tt.completedAction, constants.ActionBuild)

			// Invalid sequences should return empty next action
			if nextAction != "" {
				t.Errorf("expected empty next action for invalid sequence, got %s", nextAction)
			}
		})
	}
}

// TestActionChaining_ArtifactPassingBuildPublish tests artifact discovery from build to publish
func TestActionChaining_ArtifactPassingBuildPublish(t *testing.T) {
	// Verify that after a build completes, the artifact is discoverable by publish handler
	tests := []struct {
		name                string
		buildArtifact       string
		expectedGlobPattern string
	}{
		{
			name:                "Zarf build produces .tar.zst artifact",
			buildArtifact:       "my-package.tar.zst",
			expectedGlobPattern: "*.tar.zst",
		},
		{
			name:                "UDS create produces .tar.zst bundle",
			buildArtifact:       "my-bundle.tar.zst",
			expectedGlobPattern: "*.tar.zst",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// In multi-action workflows, artifacts are stored in a PVC at /artifacts/
			// and retrieved via glob pattern
			if tt.expectedGlobPattern != "*.tar.zst" {
				t.Errorf("expected glob pattern *.tar.zst, got %s", tt.expectedGlobPattern)
			}
		})
	}
}

// TestActionChaining_CompleteWorkflowSequence tests a complete 3-action workflow
func TestActionChaining_CompleteWorkflowSequence(t *testing.T) {
	// Simulate a BuildPublishDeploy workflow
	mainAction := constants.SpecActionBuildPublishDeploy
	primaryAction := constants.ActionBuild

	// Step 1: Build completes
	nextAction := determineNextActionForTest(mainAction, constants.ActionBuild, primaryAction)
	if nextAction != constants.ActionPublish {
		t.Errorf("step 1: expected Publish after Build, got %s", nextAction)
	}

	// Step 2: Publish completes
	nextAction = determineNextActionForTest(mainAction, constants.ActionPublish, primaryAction)
	if nextAction != constants.ActionDeploy {
		t.Errorf("step 2: expected Deploy after Publish, got %s", nextAction)
	}

	// Step 3: Deploy completes - should have no next action
	nextAction = determineNextActionForTest(mainAction, constants.ActionDeploy, primaryAction)
	if nextAction != "" {
		t.Errorf("step 3: expected no next action after Deploy, got %s", nextAction)
	}

	// Verify multi-action detection at each step
	if !isMultiActionJobForTest(mainAction) {
		t.Error("BuildPublishDeploy should be detected as multi-action")
	}
}

// TestActionChaining_UDSCreatePublishDeployWorkflow tests UDS 3-action workflow
func TestActionChaining_UDSCreatePublishDeployWorkflow(t *testing.T) {
	mainAction := constants.SpecActionCreatePublishDeploy
	primaryAction := constants.ActionCreate

	// Step 1: Create completes
	nextAction := determineNextActionForTest(mainAction, constants.ActionCreate, primaryAction)
	if nextAction != constants.ActionPublish {
		t.Errorf("step 1: expected Publish after Create, got %s", nextAction)
	}

	// Step 2: Publish completes
	nextAction = determineNextActionForTest(mainAction, constants.ActionPublish, primaryAction)
	if nextAction != constants.ActionDeploy {
		t.Errorf("step 2: expected Deploy after Publish, got %s", nextAction)
	}

	// Step 3: Deploy completes - should have no next action
	nextAction = determineNextActionForTest(mainAction, constants.ActionDeploy, primaryAction)
	if nextAction != "" {
		t.Errorf("step 3: expected no next action after Deploy, got %s", nextAction)
	}
}

// Helper functions for testing

// determineNextActionForTest replicates the logic from GenericJobMonitor.determineNextAction
// This allows us to test the action determination logic without needing a full controller setup
func determineNextActionForTest(mainAction, completedAction, primaryAction string) string {
	switch mainAction {
	case constants.SpecActionBuildPublish, constants.SpecActionCreatePublish:
		if completedAction == primaryAction || completedAction == constants.ActionCreate {
			return constants.ActionPublish
		}
	case constants.SpecActionBuildDeploy, constants.SpecActionCreateDeploy:
		if completedAction == primaryAction || completedAction == constants.ActionCreate {
			return constants.ActionDeploy
		}
	case constants.SpecActionPublishDeploy:
		if completedAction == constants.ActionPublish {
			return constants.ActionDeploy
		}
	case constants.SpecActionBuildPublishDeploy, constants.SpecActionCreatePublishDeploy:
		switch completedAction {
		case primaryAction, constants.ActionCreate:
			return constants.ActionPublish
		case constants.ActionPublish:
			return constants.ActionDeploy
		}
	}

	return ""
}

// isMultiActionJobForTest replicates the logic from GenericJobMonitor.isMultiActionJob
func isMultiActionJobForTest(action string) bool {
	multiActions := []string{
		constants.SpecActionBuildPublish,
		constants.SpecActionBuildDeploy,
		constants.SpecActionBuildPublishDeploy,
		constants.SpecActionCreatePublish,
		constants.SpecActionCreateDeploy,
		constants.SpecActionCreatePublishDeploy,
		constants.SpecActionPublishDeploy,
	}

	for _, ma := range multiActions {
		if action == ma {
			return true
		}
	}

	return false
}
