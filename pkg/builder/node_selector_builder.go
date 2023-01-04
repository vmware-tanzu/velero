/*
Copyright 2023 the Velero contributors.

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

package builder

import corev1api "k8s.io/api/core/v1"

// NodeSelectorBuilder builds NodeSelector objects
type NodeSelectorBuilder struct {
	object *corev1api.NodeSelector
}

// ForNodeSelector returns the NodeSelectorBuilder instance with given terms
func ForNodeSelector(term ...corev1api.NodeSelectorTerm) *NodeSelectorBuilder {
	return &NodeSelectorBuilder{
		object: &corev1api.NodeSelector{
			NodeSelectorTerms: term,
		},
	}
}

// Result returns the built NodeSelector
func (b *NodeSelectorBuilder) Result() *corev1api.NodeSelector {
	return b.object
}

// NodeSelectorTermBuilder builds NodeSelectorTerm objects.
type NodeSelectorTermBuilder struct {
	object *corev1api.NodeSelectorTerm
}

// NewNodeSelectorTermBuilder initializes an instance of NodeSelectorTermBuilder
func NewNodeSelectorTermBuilder() *NodeSelectorTermBuilder {
	return &NodeSelectorTermBuilder{
		object: &corev1api.NodeSelectorTerm{
			MatchExpressions: make([]corev1api.NodeSelectorRequirement, 0),
			MatchFields:      make([]corev1api.NodeSelectorRequirement, 0),
		},
	}
}

// WithMatchExpression appends the MatchExpression to the NodeSelectorTerm
func (ntb *NodeSelectorTermBuilder) WithMatchExpression(key string, op string, values ...string) *NodeSelectorTermBuilder {
	req := corev1api.NodeSelectorRequirement{
		Key:      key,
		Operator: corev1api.NodeSelectorOperator(op),
		Values:   values,
	}
	ntb.object.MatchExpressions = append(ntb.object.MatchExpressions, req)
	return ntb
}

// WithMatchField appends the MatchField to the NodeSelectorTerm
func (ntb *NodeSelectorTermBuilder) WithMatchField(key string, op string, values ...string) *NodeSelectorTermBuilder {
	req := corev1api.NodeSelectorRequirement{
		Key:      key,
		Operator: corev1api.NodeSelectorOperator(op),
		Values:   values,
	}
	ntb.object.MatchFields = append(ntb.object.MatchFields, req)
	return ntb
}

// Result returns the built NodeSelectorTerm
func (ntb *NodeSelectorTermBuilder) Result() *corev1api.NodeSelectorTerm {
	return ntb.object
}
