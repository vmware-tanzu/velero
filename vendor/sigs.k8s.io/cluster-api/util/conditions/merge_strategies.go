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
	"strings"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

// mergeOptions allows to set strategies for merging a set of conditions into a single condition,
// and more specifically for computing the target Reason and the target Message.
type mergeOptions struct {
	conditionTypes                     []clusterv1.ConditionType
	addSourceRef                       bool
	addStepCounter                     bool
	addStepCounterIfOnlyConditionTypes []clusterv1.ConditionType
	stepCounter                        int
}

// MergeOption defines an option for computing a summary of conditions.
type MergeOption func(*mergeOptions)

// WithConditions instructs merge about the condition types to consider when doing a merge operation;
// if this option is not specified, all the conditions (excepts Ready) will be considered. This is required
// so we can provide some guarantees about the semantic of the target condition without worrying about
// side effects if someone or something adds custom conditions to the objects.
//
// NOTE: The order of conditions types defines the priority for determining the Reason and Message for the
// target condition.
// IMPORTANT: This options works only while generating the Summary condition.
func WithConditions(t ...clusterv1.ConditionType) MergeOption {
	return func(c *mergeOptions) {
		c.conditionTypes = t
	}
}

// WithStepCounter instructs merge to add a "x of y completed" string to the message,
// where x is the number of conditions with Status=true and y is the number of conditions in scope.
func WithStepCounter() MergeOption {
	return func(c *mergeOptions) {
		c.addStepCounter = true
	}
}

// If it is required to add a step counter only if a subset of condition exists, check if the conditions
// in scope are included in this subset. This applies for example on Machines, where we want to use
// the step counter notation while provisioning the machine, but then we want to move away from this notation
// as soon as the machine is provisioned and e.g. a Machine health check condition is generated
//
// IMPORTANT: This options works only while generating the Summary condition.
func WithStepCounterIfOnly(t ...clusterv1.ConditionType) MergeOption {
	return func(c *mergeOptions) {
		c.addStepCounterIfOnlyConditionTypes = t
	}
}

// AddSourceRef instructs merge to add info about the originating object to the target Reason.
func AddSourceRef() MergeOption {
	return func(c *mergeOptions) {
		c.addSourceRef = true
	}
}

// getReason returns the reason to be applied to the condition resulting by merging a set of condition groups.
// The reason is computed according to the given mergeOptions.
func getReason(groups conditionGroups, options *mergeOptions) string {
	return getFirstReason(groups, options.conditionTypes, options.addSourceRef)
}

// getFirstReason returns the first reason from the ordered list of conditions in the top group.
// If required, the reason gets localized with the source object reference.
func getFirstReason(g conditionGroups, order []clusterv1.ConditionType, addSourceRef bool) string {
	if condition := getFirstCondition(g, order); condition != nil {
		reason := condition.Reason
		if addSourceRef {
			return localizeReason(reason, condition.Getter)
		}
		return reason
	}
	return ""
}

// localizeReason adds info about the originating object to the target Reason.
func localizeReason(reason string, from Getter) string {
	if strings.Contains(reason, "@") {
		return reason
	}
	return fmt.Sprintf("%s@%s/%s", reason, from.GetObjectKind().GroupVersionKind().Kind, from.GetName())
}

// getMessage returns the message to be applied to the condition resulting by merging a set of condition groups.
// The message is computed according to the given mergeOptions, but in case of errors or warning a
// summary of existing errors is automatically added.
func getMessage(groups conditionGroups, options *mergeOptions) string {
	if options.addStepCounter {
		return getStepCounterMessage(groups, options.stepCounter)
	}

	return getFirstMessage(groups, options.conditionTypes)
}

// getStepCounterMessage returns a message "x of y completed", where x is the number of conditions
// with Status=true and y is the number passed to this method.
func getStepCounterMessage(groups conditionGroups, to int) string {
	ct := 0
	if trueGroup := groups.TrueGroup(); trueGroup != nil {
		ct = len(trueGroup.conditions)
	}
	return fmt.Sprintf("%d of %d completed", ct, to)
}

// getFirstMessage returns the message from the ordered list of conditions in the top group.
func getFirstMessage(groups conditionGroups, order []clusterv1.ConditionType) string {
	if condition := getFirstCondition(groups, order); condition != nil {
		return condition.Message
	}
	return ""
}

// getFirstCondition returns a first condition from the ordered list of conditions in the top group.
func getFirstCondition(g conditionGroups, priority []clusterv1.ConditionType) *localizedCondition {
	topGroup := g.TopGroup()
	if topGroup == nil {
		return nil
	}

	switch len(topGroup.conditions) {
	case 0:
		return nil
	case 1:
		return &topGroup.conditions[0]
	default:
		for _, p := range priority {
			for _, c := range topGroup.conditions {
				if c.Type == p {
					return &c
				}
			}
		}
		return &topGroup.conditions[0]
	}
}
