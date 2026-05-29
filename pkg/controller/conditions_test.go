package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kylegalloway/forge/pkg/constants"
)

func TestDeriveConditions_Ready(t *testing.T) {
	cases := []struct {
		phase           string
		wantReadyStatus metav1.ConditionStatus
		wantReconciling metav1.ConditionStatus
	}{
		{constants.PhaseCompleted, metav1.ConditionTrue, metav1.ConditionFalse},
		{constants.PhaseFailed, metav1.ConditionFalse, metav1.ConditionFalse},
		{constants.PhaseRunning, metav1.ConditionUnknown, metav1.ConditionTrue},
		{constants.PhasePending, metav1.ConditionUnknown, metav1.ConditionTrue},
		{constants.PhaseRetrying, metav1.ConditionUnknown, metav1.ConditionTrue},
		{constants.PhaseQueued, metav1.ConditionUnknown, metav1.ConditionTrue},
	}
	for _, tc := range cases {
		t.Run(tc.phase, func(t *testing.T) {
			conds := DeriveConditions(tc.phase, map[string]interface{}{}, nil, 1)
			ready := findCondition(conds, constants.ConditionTypeReady)
			if ready == nil {
				t.Fatal("Ready condition missing")
			}
			if ready.Status != tc.wantReadyStatus {
				t.Errorf("Ready status: got %s, want %s", ready.Status, tc.wantReadyStatus)
			}
			reconciling := findCondition(conds, constants.ConditionTypeReconciling)
			if reconciling == nil {
				t.Fatal("Reconciling condition missing")
			}
			if reconciling.Status != tc.wantReconciling {
				t.Errorf("Reconciling status: got %s, want %s", reconciling.Status, tc.wantReconciling)
			}
		})
	}
}

func TestDeriveConditions_OperationConditions(t *testing.T) {
	status := map[string]interface{}{
		"buildStatus": map[string]interface{}{
			constants.StatusKeyState: constants.PhaseCompleted,
		},
		"publishStatus": map[string]interface{}{
			constants.StatusKeyState: constants.PhaseRunning,
		},
	}
	conds := DeriveConditions(constants.PhaseRunning, status, nil, 1)

	build := findCondition(conds, constants.ConditionTypeBuildSucceeded)
	if build == nil {
		t.Fatal("BuildSucceeded condition missing")
	}
	if build.Status != metav1.ConditionTrue {
		t.Errorf("BuildSucceeded: got %s, want True", build.Status)
	}

	publish := findCondition(conds, constants.ConditionTypePublishSucceeded)
	if publish == nil {
		t.Fatal("PublishSucceeded condition missing")
	}
	if publish.Status != metav1.ConditionUnknown {
		t.Errorf("PublishSucceeded: got %s, want Unknown", publish.Status)
	}

	// deployStatus not in status map — condition should not appear
	deploy := findCondition(conds, constants.ConditionTypeDeploySucceeded)
	if deploy != nil {
		t.Error("DeploySucceeded should not be present when deployStatus is absent")
	}
}

func TestDeriveConditions_PreservesLastTransitionTime(t *testing.T) {
	past := metav1.NewTime(metav1.Now().Add(-60e9))
	existing := []metav1.Condition{
		{
			Type:               constants.ConditionTypeReady,
			Status:             metav1.ConditionUnknown,
			LastTransitionTime: past,
			Reason:             constants.ConditionReasonProgressing,
		},
	}
	// Same status (Unknown → Unknown): LastTransitionTime must be preserved.
	conds := DeriveConditions(constants.PhaseRunning, map[string]interface{}{}, existing, 1)
	ready := findCondition(conds, constants.ConditionTypeReady)
	if ready == nil {
		t.Fatal("Ready condition missing")
	}
	if !ready.LastTransitionTime.Equal(&past) {
		t.Errorf("LastTransitionTime changed when status was unchanged: got %v, want %v", ready.LastTransitionTime, past)
	}
}

func TestDeriveConditions_TransitionsLastTransitionTime(t *testing.T) {
	past := metav1.NewTime(metav1.Now().Add(-60e9))
	existing := []metav1.Condition{
		{
			Type:               constants.ConditionTypeReady,
			Status:             metav1.ConditionUnknown,
			LastTransitionTime: past,
			Reason:             constants.ConditionReasonProgressing,
		},
	}
	// Status changes (Unknown → True): LastTransitionTime must be updated.
	conds := DeriveConditions(constants.PhaseCompleted, map[string]interface{}{}, existing, 1)
	ready := findCondition(conds, constants.ConditionTypeReady)
	if ready == nil {
		t.Fatal("Ready condition missing")
	}
	if ready.LastTransitionTime.Equal(&past) {
		t.Error("LastTransitionTime should have been updated on status transition")
	}
}

func TestConditionsRoundtrip(t *testing.T) {
	original := []metav1.Condition{
		{
			Type:               constants.ConditionTypeReady,
			Status:             metav1.ConditionTrue,
			Reason:             constants.ConditionReasonSucceeded,
			ObservedGeneration: 3,
			LastTransitionTime: metav1.Now(),
		},
	}
	roundtripped := conditionsFromUnstructured(conditionsToUnstructured(original))
	if len(roundtripped) != 1 {
		t.Fatalf("expected 1 condition after roundtrip, got %d", len(roundtripped))
	}
	if roundtripped[0].Type != original[0].Type {
		t.Errorf("Type: got %s, want %s", roundtripped[0].Type, original[0].Type)
	}
	if roundtripped[0].Status != original[0].Status {
		t.Errorf("Status: got %s, want %s", roundtripped[0].Status, original[0].Status)
	}
	if roundtripped[0].ObservedGeneration != original[0].ObservedGeneration {
		t.Errorf("ObservedGeneration: got %d, want %d", roundtripped[0].ObservedGeneration, original[0].ObservedGeneration)
	}
}

func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
