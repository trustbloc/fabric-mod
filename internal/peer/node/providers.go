/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/peer"
	"github.com/hyperledger/fabric/gossip/service"
	"github.com/hyperledger/fabric/msp"
	"github.com/hyperledger/fabric/msp/mgmt"
)

func newGossipProvider() service.GossipService {
	return service.GetGossipService()
}

type ledgerProvider struct {
}

func newLedgerProvider() *ledgerProvider {
	return &ledgerProvider{}
}

// GetLedger returns the ledger for the given channel
func (p *ledgerProvider) GetLedger(channelID string) ledger.PeerLedger {
	return peer.GetLedger(channelID)
}

type mspProvider struct {
	msp.MSP
}

func newMSPProvider() *mspProvider {
	return &mspProvider{
		MSP: mgmt.GetLocalMSP(),
	}
}

// GetIdentityDeserializer returns the identity deserializer for the given channel
func (m *mspProvider) GetIdentityDeserializer(channelID string) msp.IdentityDeserializer {
	return mgmt.GetIdentityDeserializer(channelID)
}
