package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kylegalloway/forge/pkg/constants"
)

// DeriveConditions projects the phase state machine into a []metav1.Condition slice.
// existing is the prior conditions slice, used to preserve LastTransitionTime when
// a condition's status has not changed.
func DeriveConditions(phase string, status map[string]interface{}, existing []metav1.Condition, generation int64) []metav1.Condition {
	now := metav1.Now()
	conditions := make([]metav1.Condition, 0, 6)

	conditions = setCondition(conditions, existing, readyCondition(phase, generation, now))
	conditions = setCondition(conditions, existing, reconcilingCondition(phase, generation, now))

	for field, condType := range constants.OperationConditionTypes {
		opStatus, ok := status[field].(map[string]interface{})
		if !ok {
			continue
		}
		opPhase, ok := opStatus[constants.StatusKeyState].(string)
		if !ok || opPhase == "" {
			continue
		}
		conditions = setCondition(conditions, existing, operationCondition(condType, opPhase, generation, now))
	}

	return conditions
}

func readyCondition(phase string, generation int64, now metav1.Time) metav1.Condition {
	c := metav1.Condition{
		Type:               constants.ConditionTypeReady,
		ObservedGeneration: generation,
		LastTransitionTime: now,
	}
	switch phase {
	case constants.PhaseCompleted:
		c.Status = metav1.ConditionTrue
		c.Reason = constants.ConditionReasonSucceeded
	case constants.PhaseFailed:
		c.Status = metav1.ConditionFalse
		c.Reason = constants.ConditionReasonFailed
	default:
		c.Status = metav1.ConditionUnknown
		c.Reason = constants.ConditionReasonProgressing
	}
	return c
}

func reconcilingCondition(phase string, generation int64, now metav1.Time) metav1.Condition {
	terminal := phase == constants.PhaseCompleted || phase == constants.PhaseFailed
	status := metav1.ConditionTrue
	if terminal {
		status = metav1.ConditionFalse
	}
	return metav1.Condition{
		Type:               constants.ConditionTypeReconciling,
		Status:             status,
		Reason:             constants.ConditionReasonProgressing,
		ObservedGeneration: generation,
		LastTransitionTime: now,
	}
}

func operationCondition(condType, phase string, generation int64, now metav1.Time) metav1.Condition {
	c := metav1.Condition{
		Type:               condType,
		ObservedGeneration: generation,
		LastTransitionTime: now,
	}
	switch phase {
	case constants.PhaseCompleted:
		c.Status = metav1.ConditionTrue
		c.Reason = constants.ConditionReasonSucceeded
	case constants.PhaseFailed:
		c.Status = metav1.ConditionFalse
		c.Reason = constants.ConditionReasonFailed
	default:
		c.Status = metav1.ConditionUnknown
		c.Reason = constants.ConditionReasonProgressing
	}
	return c
}

// setCondition appends or replaces a condition, preserving LastTransitionTime
// when the status is unchanged.
func setCondition(conditions []metav1.Condition, existing []metav1.Condition, c metav1.Condition) []metav1.Condition {
	for _, e := range existing {
		if e.Type == c.Type && e.Status == c.Status {
			c.LastTransitionTime = e.LastTransitionTime
			break
		}
	}
	for i, cur := range conditions {
		if cur.Type == c.Type {
			conditions[i] = c
			return conditions
		}
	}
	return append(conditions, c)
}

// conditionsToUnstructured converts a []metav1.Condition to the []interface{}
// form required for unstructured status writes.
func conditionsToUnstructured(conditions []metav1.Condition) []interface{} {
	out := make([]interface{}, 0, len(conditions))
	for i := range conditions {
		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&conditions[i])
		if err != nil {
			continue
		}
		out = append(out, m)
	}
	return out
}

// conditionsFromUnstructured reads the conditions slice from a raw status map.
func conditionsFromUnstructured(raw interface{}) []metav1.Condition {
	list, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	conditions := make([]metav1.Condition, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		var c metav1.Condition
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(m, &c); err != nil {
			continue
		}
		conditions = append(conditions, c)
	}
	return conditions
}
