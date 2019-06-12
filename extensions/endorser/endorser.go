/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package endorser

import (
	"github.com/hyperledger/fabric/extensions/endorser/api"
	"github.com/hyperledger/fabric/protos/ledger/rwset"
)

// CollRWSetFilter filters out all off-ledger (including transient data) read-write sets from the simulation results
// so that they won't be included in the block.
type CollRWSetFilter interface {
	Filter(channelID string, pubSimulationResults *rwset.TxReadWriteSet) (*rwset.TxReadWriteSet, error)
}

// NewCollRWSetFilter returns a new collection RW set filter
func NewCollRWSetFilter(api.QueryExecutorProviderFactory, api.BlockPublisherProvider) CollRWSetFilter {
	return &collRWSetFilter{}
}

type collRWSetFilter struct {
}

// Filter is a noop filter. It simply returns the passed in r/w set
func (f *collRWSetFilter) Filter(channelID string, pubSimulationResults *rwset.TxReadWriteSet) (*rwset.TxReadWriteSet, error) {
	return pubSimulationResults, nil
}
