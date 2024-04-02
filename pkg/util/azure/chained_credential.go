package azure

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// This file is a copy of https://github.com/Azure/azure-sdk-for-go/blob/sdk/azidentity/v1.5.1/sdk/azidentity/chained_token_credential.go with a change that
// removes the specific error checking logic.
// Velero chains the ConfigCredential, WorkloadIdentityCredential, ManagedIdentityCredential and uses them to do the auth in order.
// The original ChainedTokenCredential only reports the error got from the last credential and ignores the others in some cases, this causes confusion
// if the root cause is from the former credentials.
// For example, if users provide an invalid certificate as the credential, the original reports a managed identity credential error, this is hard to debug.
// With the change in this file, the new ChainedTokenCredential reports all errors of all of the credentials.

// ChainedTokenCredential links together multiple credentials and tries them sequentially when authenticating. By default,
// it tries all the credentials until one authenticates, after which it always uses that credential.
type ChainedTokenCredential struct {
	cond                 *sync.Cond
	iterating            bool
	name                 string
	retrySources         bool
	sources              []azcore.TokenCredential
	successfulCredential azcore.TokenCredential
}

// NewChainedTokenCredential creates a ChainedTokenCredential. Pass nil for options to accept defaults.
func NewChainedTokenCredential(sources []azcore.TokenCredential, options *azidentity.ChainedTokenCredentialOptions) (*ChainedTokenCredential, error) {
	if len(sources) == 0 {
		return nil, errors.New("sources must contain at least one TokenCredential")
	}
	for _, source := range sources {
		if source == nil { // cannot have a nil credential in the chain or else the application will panic when GetToken() is called on nil
			return nil, errors.New("sources cannot contain nil")
		}
	}
	cp := make([]azcore.TokenCredential, len(sources))
	copy(cp, sources)
	if options == nil {
		options = &azidentity.ChainedTokenCredentialOptions{}
	}
	return &ChainedTokenCredential{
		cond:         sync.NewCond(&sync.Mutex{}),
		name:         "ChainedTokenCredential",
		retrySources: options.RetrySources,
		sources:      cp,
	}, nil
}

// GetToken calls GetToken on the chained credentials in turn, stopping when one returns a token.
// This method is called automatically by Azure SDK clients.
func (c *ChainedTokenCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	if !c.retrySources {
		// ensure only one goroutine at a time iterates the sources and perhaps sets c.successfulCredential
		c.cond.L.Lock()
		for {
			if c.successfulCredential != nil {
				c.cond.L.Unlock()
				return c.successfulCredential.GetToken(ctx, opts)
			}
			if !c.iterating {
				c.iterating = true
				// allow other goroutines to wait while this one iterates
				c.cond.L.Unlock()
				break
			}
			c.cond.Wait()
		}
	}

	var (
		err                  error
		errs                 []error
		successfulCredential azcore.TokenCredential
		token                azcore.AccessToken
	)
	for _, cred := range c.sources {
		token, err = cred.GetToken(ctx, opts)
		if err == nil {
			successfulCredential = cred
			break
		}
		errs = append(errs, err)
	}
	if c.iterating {
		c.cond.L.Lock()
		// this is nil when all credentials returned an error
		c.successfulCredential = successfulCredential
		c.iterating = false
		c.cond.L.Unlock()
		c.cond.Broadcast()
	}
	// err is the error returned by the last GetToken call. It will be nil when that call succeeds
	if err != nil {
		msg := createChainedErrorMessage(errs)
		err = fmt.Errorf("%s", msg)
	}
	return token, err
}

func createChainedErrorMessage(errs []error) string {
	msg := "failed to acquire a token.\nAttempted credentials:"
	for _, err := range errs {
		msg += fmt.Sprintf("\n\t%s", err.Error())
	}
	return msg
}
