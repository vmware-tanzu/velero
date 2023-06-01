package flag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStringOfLabelSelector(t *testing.T) {
	ls, err := metav1.ParseToLabelSelector("k1=v1,k2=v2")
	require.Nil(t, err)
	selector := &LabelSelector{
		LabelSelector: ls,
	}
	assert.Equal(t, "k1=v1,k2=v2", selector.String())
}

func TestSetOfLabelSelector(t *testing.T) {
	selector := &LabelSelector{}
	require.Nil(t, selector.Set("k1=v1,k2=v2"))
	str := selector.String()
	assert.True(t, str == "k1=v1,k2=v2" || str == "k2=v2,k2=v2")
}

func TestTypeOfLabelSelector(t *testing.T) {
	selector := &LabelSelector{}
	assert.Equal(t, "labelSelector", selector.Type())
}
