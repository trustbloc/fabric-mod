/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package mocks

import (
	"github.com/hyperledger/fabric/extensions/gossip/api"
	cb "github.com/hyperledger/fabric/protos/common"
)

// BlockPublisher is a mock block publisher
type BlockPublisher struct {
}

// NewBlockPublisher returns a new mock block publisher
func NewBlockPublisher() *BlockPublisher {
	return &BlockPublisher{}
}

// AddCCUpgradeHandler adds a handler for chaincode upgrades
func (m *BlockPublisher) AddCCUpgradeHandler(handler api.ChaincodeUpgradeHandler) {
	// Not implemented
}

// AddConfigUpdateHandler adds a handler for config updates
func (m *BlockPublisher) AddConfigUpdateHandler(handler api.ConfigUpdateHandler) {
	// Not implemented
}

// AddWriteHandler adds a handler for KV writes
func (m *BlockPublisher) AddWriteHandler(handler api.WriteHandler) {
	// Not implemented
}

// AddReadHandler adds a handler for KV reads
func (m *BlockPublisher) AddReadHandler(handler api.ReadHandler) {
	// Not implemented
}

// AddCCEventHandler adds a handler for chaincode events
func (m *BlockPublisher) AddCCEventHandler(handler api.ChaincodeEventHandler) {
	// Not implemented
}

// Publish traverses the block and invokes all applicable handlers
func (m *BlockPublisher) Publish(block *cb.Block) {
	// Not implemented
}
