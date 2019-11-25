/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"github.com/hyperledger/fabric/bccsp/factory"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/cceventmgmt"
	ccapi "github.com/hyperledger/fabric/extensions/chaincode/api"
	"github.com/hyperledger/fabric/extensions/gossip/api"
	gossipservice "github.com/hyperledger/fabric/gossip/service"
	"github.com/hyperledger/fabric/msp"
	"github.com/hyperledger/fabric/msp/mgmt"
)

type gossipProvider struct {
	service *gossipservice.GossipService
}

func newGossipProvider(service *gossipservice.GossipService) *gossipProvider {
	return &gossipProvider{service: service}
}

// GetGossipService returns the Gossip service
func (p *gossipProvider) GetGossipService() api.GossipService {
	return p.service
}

type mspProvider struct {
	msp.MSP
}

func newMSPProvider() *mspProvider {
	return &mspProvider{
		MSP: mgmt.GetLocalMSP(factory.GetDefault()),
	}
}

// GetIdentityDeserializer returns the identity deserializer for the given channel
func (m *mspProvider) GetIdentityDeserializer(channelID string) msp.IdentityDeserializer {
	return mgmt.GetIdentityDeserializer(channelID, factory.GetDefault())
}

type ledgerConfigProvider struct {
	config *ledger.Config
}

func newLedgerConfigProvider(config *ledger.Config) *ledgerConfigProvider {
	return &ledgerConfigProvider{config: config}
}

// GetLedgerConfig returns the ledger configuration
func (p *ledgerConfigProvider) GetLedgerConfig() *ledger.Config {
	return p.config
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
