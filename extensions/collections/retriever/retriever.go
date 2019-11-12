/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package retriever

import (
	storeapi "github.com/hyperledger/fabric/extensions/collections/api/store"
)

// Provider is a private data provider.
type Provider struct {
}

// NewProvider returns a new private data provider
func NewProvider() *Provider {
	return &Provider{}
}

// Initialize initializes the provider
func (p *Provider) Initialize() *Provider {
	// Noop
	return p
}

// RetrieverForChannel returns the private data dataRetriever for the given channel
func (p *Provider) RetrieverForChannel(channelID string) storeapi.Retriever {
	panic("not implemented")
}
