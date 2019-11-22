/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/cceventmgmt"
	"github.com/hyperledger/fabric/core/peer"
	ccapi "github.com/hyperledger/fabric/extensions/chaincode/api"
	"github.com/hyperledger/fabric/extensions/gossip/api"
	"github.com/hyperledger/fabric/gossip/service"
	"github.com/hyperledger/fabric/msp"
	"github.com/hyperledger/fabric/msp/mgmt"
)

type gossipProvider struct {
}

func newGossipProvider() *gossipProvider {
	return &gossipProvider{}
}

// GetGossipService returns the Gossip service
func (p *gossipProvider) GetGossipService() api.GossipService {
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

type ccEventMgr struct {
}

// HandleChaincodeDeploy delegates to the chaincode event manager.
// The ChaincodeDefinition struct was redefined in order to remove dependencies on the cceventmgmt package
// which would cause circular import problems.
func (m *ccEventMgr) HandleChaincodeDeploy(channelID string, ccDefs []*ccapi.Definition) error {
	chaincodeDefs := make([]*cceventmgmt.ChaincodeDefinition, len(ccDefs))
	for i, ccDef := range ccDefs {
		chaincodeDefs[i] = &cceventmgmt.ChaincodeDefinition{
			Name:              ccDef.Name,
			Hash:              ccDef.Hash,
			Version:           ccDef.Version,
			CollectionConfigs: ccDef.CollectionConfigs,
		}
	}
	return cceventmgmt.GetMgr().HandleChaincodeDeploy(channelID, chaincodeDefs)
}

// ChaincodeDeployDone delegates to the chaincode event manager.
func (m *ccEventMgr) ChaincodeDeployDone(channelID string) {
	cceventmgmt.GetMgr().ChaincodeDeployDone(channelID)
}

type ccEventMgrProvider struct {
	mgr *ccEventMgr
}

func newCCEventMgrProvider() *ccEventMgrProvider {
	return &ccEventMgrProvider{mgr: &ccEventMgr{}}
}

// GetMgr returns the chaincode event manager
func (p *ccEventMgrProvider) GetMgr() ccapi.EventMgr {
	return p.mgr
}
