package azure

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChainedTokenCredential(t *testing.T) {
	// empty source
	var sources []azcore.TokenCredential
	_, err := NewChainedTokenCredential(sources, nil)
	require.NotNil(t, err)

	// contain nil source
	sources = append(sources, nil)
	_, err = NewChainedTokenCredential(sources, nil)
	require.NotNil(t, err)

	// valid
	sources = []azcore.TokenCredential{&credentialErrorReporter{}}
	credential, err := NewChainedTokenCredential(sources, nil)
	require.Nil(t, err)
	assert.NotNil(t, credential)
}

func TestGetToken(t *testing.T) {
	sources := []azcore.TokenCredential{&credentialErrorReporter{err: &credentialError{
		credType: "fake",
		err:      errors.New("fake error"),
	}}}
	credential, err := NewChainedTokenCredential(sources, nil)
	require.Nil(t, err)

	_, err = credential.GetToken(context.Background(), policy.TokenRequestOptions{})
	require.NotNil(t, err)
}
