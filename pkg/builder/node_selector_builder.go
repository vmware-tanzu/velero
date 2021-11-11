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
