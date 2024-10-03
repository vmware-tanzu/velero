package flag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStringOfOrLabelSelector(t *testing.T) {
	tests := []struct {
		name            string
		orLabelSelector *OrLabelSelector
		expectedStr     string
	}{
		{
			name: "or between two labels",
			orLabelSelector: &OrLabelSelector{
				OrLabelSelectors: []*metav1.LabelSelector{
					{
						MatchLabels: map[string]string{"k1": "v1"},
					},
					{
						MatchLabels: map[string]string{"k2": "v2"},
					},
				},
			},
			expectedStr: "k1=v1 or k2=v2",
		},
		{
			name: "or between two label groups",
			orLabelSelector: &OrLabelSelector{
				OrLabelSelectors: []*metav1.LabelSelector{
					{
						MatchLabels: map[string]string{"k1": "v1", "k2": "v2"},
					},
					{
						MatchLabels: map[string]string{"a1": "b1", "a2": "b2"},
					},
				},
			},
			expectedStr: "k1=v1,k2=v2 or a1=b1,a2=b2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedStr, test.orLabelSelector.String())
		})
	}
}

func TestSetOfOrLabelSelector(t *testing.T) {
	tests := []struct {
		name             string
		inputStr         string
		expectedSelector *OrLabelSelector
	}{
		{
			name:     "or between two labels",
			inputStr: "k1=v1 or k2=v2",
			expectedSelector: &OrLabelSelector{
				OrLabelSelectors: []*metav1.LabelSelector{
					{
						MatchLabels: map[string]string{"k1": "v1"},
					},
					{
						MatchLabels: map[string]string{"k2": "v2"},
					},
				},
			},
		},
		{
			name:     "or between two label groups",
			inputStr: "k1=v1,k2=v2 or a1=b1,a2=b2",
			expectedSelector: &OrLabelSelector{
				OrLabelSelectors: []*metav1.LabelSelector{
					{
						MatchLabels: map[string]string{"k1": "v1", "k2": "v2"},
					},
					{
						MatchLabels: map[string]string{"a1": "b1", "a2": "b2"},
					},
				},
			},
		},
	}
	selector := &OrLabelSelector{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, selector.Set(test.inputStr))
			assert.Equal(t, len(test.expectedSelector.OrLabelSelectors), len(selector.OrLabelSelectors))
			assert.Equal(t, test.expectedSelector.String(), selector.String())
		})
	}
}

func TestTypeOfOrLabelSelector(t *testing.T) {
	selector := &OrLabelSelector{}
	assert.Equal(t, "orLabelSelector", selector.Type())
}
