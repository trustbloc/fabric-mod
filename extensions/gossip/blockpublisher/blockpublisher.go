/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockpublisher

import (
	"github.com/hyperledger/fabric/extensions/gossip/api"
	cb "github.com/hyperledger/fabric/protos/common"
)

// ProviderInstance manages a block publisher for each channel
var ProviderInstance = &Provider{}

// ForChannel returns the block publisher for the given channel
func ForChannel(channelID string) api.BlockPublisher {
	return ProviderInstance.ForChannel(channelID)
}

// Provider is a noop block publisher provider
type Provider struct {
}

// ForChannel returns a noop publisher
func (p *Provider) ForChannel(channelID string) api.BlockPublisher {
	return &publisher{}
}

// Close does nothing
func (p *Provider) Close() {
	// Nothing to do
}

type publisher struct {
}

func (p *publisher) AddCCUpgradeHandler(handler api.ChaincodeUpgradeHandler) {
	// Not implemented
}

func (p *publisher) AddConfigUpdateHandler(handler api.ConfigUpdateHandler) {
	// Not implemented
}

func (p *publisher) AddWriteHandler(handler api.WriteHandler) {
	// Not implemented
}

func (p *publisher) AddReadHandler(handler api.ReadHandler) {
	// Not implemented
}

func (p *publisher) AddLSCCWriteHandler(handler api.LSCCWriteHandler) {
	// Not implemented
}

func (p *publisher) AddCCEventHandler(handler api.ChaincodeEventHandler) {
	// Not implemented
}

func (p *publisher) Publish(block *cb.Block) {
	// Not implemented
}

func (p *publisher) LedgerHeight() uint64 {
	// Not implemented
	return 0
}
