/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package conditions

import (
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

// Setter interface defines methods that a Cluster API object should implement in order to
// use the conditions package for setting conditions.
type Setter interface {
	Getter
	SetConditions(clusterv1.Conditions)
}

// Set sets the given condition.
//
// NOTE: If a condition already exists, the LastTransitionTime is updated only if a change is detected
// in any of the following fields: Status, Reason, Severity and Message.
func Set(to Setter, condition *clusterv1.Condition) {
	if to == nil || condition == nil {
		return
	}

	// Check if the new conditions already exists, and change it only if there is a status
	// transition (otherwise we should preserve the current last transition time)-
	conditions := to.GetConditions()
	exists := false
	for i := range conditions {
		existingCondition := conditions[i]
		if existingCondition.Type == condition.Type {
			exists = true
			if !hasSameState(&existingCondition, condition) {
				condition.LastTransitionTime = metav1.NewTime(time.Now().UTC().Truncate(time.Second))
				conditions[i] = *condition
				break
			}
			condition.LastTransitionTime = existingCondition.LastTransitionTime
			break
		}
	}

	// If the condition does not exist, add it, setting the transition time only if not already set
	if !exists {
		if condition.LastTransitionTime.IsZero() {
			condition.LastTransitionTime = metav1.NewTime(time.Now().UTC().Truncate(time.Second))
		}
		conditions = append(conditions, *condition)
	}

	// Sorts conditions for convenience of the consumer, i.e. kubectl.
	sort.Slice(conditions, func(i, j int) bool {
		return lexicographicLess(&conditions[i], &conditions[j])
	})

	to.SetConditions(conditions)
}

// TrueCondition returns a condition with Status=True and the given type.
func TrueCondition(t clusterv1.ConditionType) *clusterv1.Condition {
	return &clusterv1.Condition{
		Type:   t,
		Status: corev1.ConditionTrue,
	}
}

// FalseCondition returns a condition with Status=False and the given type.
func FalseCondition(t clusterv1.ConditionType, reason string, severity clusterv1.ConditionSeverity, messageFormat string, messageArgs ...interface{}) *clusterv1.Condition {
	return &clusterv1.Condition{
		Type:     t,
		Status:   corev1.ConditionFalse,
		Reason:   reason,
		Severity: severity,
		Message:  fmt.Sprintf(messageFormat, messageArgs...),
	}
}

// UnknownCondition returns a condition with Status=Unknown and the given type.
func UnknownCondition(t clusterv1.ConditionType, reason string, messageFormat string, messageArgs ...interface{}) *clusterv1.Condition {
	return &clusterv1.Condition{
		Type:    t,
		Status:  corev1.ConditionUnknown,
		Reason:  reason,
		Message: fmt.Sprintf(messageFormat, messageArgs...),
	}
}

// MarkTrue sets Status=True for the condition with the given type.
func MarkTrue(to Setter, t clusterv1.ConditionType) {
	Set(to, TrueCondition(t))
}

// MarkUnknown sets Status=Unknown for the condition with the given type.
func MarkUnknown(to Setter, t clusterv1.ConditionType, reason, messageFormat string, messageArgs ...interface{}) {
	Set(to, UnknownCondition(t, reason, messageFormat, messageArgs...))
}

// MarkFalse sets Status=False for the condition with the given type.
func MarkFalse(to Setter, t clusterv1.ConditionType, reason string, severity clusterv1.ConditionSeverity, messageFormat string, messageArgs ...interface{}) {
	Set(to, FalseCondition(t, reason, severity, messageFormat, messageArgs...))
}

// SetSummary sets a Ready condition with the summary of all the conditions existing
// on an object. If the object does not have other conditions, no summary condition is generated.
func SetSummary(to Setter, options ...MergeOption) {
	Set(to, summary(to, options...))
}

// SetMirror creates a new condition by mirroring the the Ready condition from a dependent object;
// if the Ready condition does not exists in the source object, no target conditions is generated.
func SetMirror(to Setter, targetCondition clusterv1.ConditionType, from Getter, options ...MirrorOptions) {
	Set(to, mirror(from, targetCondition, options...))
}

// SetAggregate creates a new condition with the aggregation of all the the Ready condition
// from a list of dependent objects; if the Ready condition does not exists in one of the source object,
// the object is excluded from the aggregation; if none of the source object have ready condition,
// no target conditions is generated.
func SetAggregate(to Setter, targetCondition clusterv1.ConditionType, from []Getter, options ...MergeOption) {
	Set(to, aggregate(from, targetCondition, options...))
}

// Delete deletes the condition with the given type.
func Delete(to Setter, t clusterv1.ConditionType) {
	if to == nil {
		return
	}

	conditions := to.GetConditions()
	newConditions := make(clusterv1.Conditions, 0, len(conditions))
	for _, condition := range conditions {
		if condition.Type != t {
			newConditions = append(newConditions, condition)
		}
	}
	to.SetConditions(newConditions)
}

// lexicographicLess returns true if a condition is less than another with regards to the
// to order of conditions designed for convenience of the consumer, i.e. kubectl.
// According to this order the Ready condition always goes first, followed by all the other
// conditions sorted by Type.
func lexicographicLess(i, j *clusterv1.Condition) bool {
	return (i.Type == clusterv1.ReadyCondition || i.Type < j.Type) && j.Type != clusterv1.ReadyCondition
}

// hasSameState returns true if a condition has the same state of another; state is defined
// by the union of following fields: Type, Status, Reason, Severity and Message (it excludes LastTransitionTime).
func hasSameState(i, j *clusterv1.Condition) bool {
	return i.Type == j.Type &&
		i.Status == j.Status &&
		i.Reason == j.Reason &&
		i.Severity == j.Severity &&
		i.Message == j.Message
}
