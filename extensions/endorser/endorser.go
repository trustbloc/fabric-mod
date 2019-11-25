/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package endorser

import (
	"github.com/hyperledger/fabric-protos-go/ledger/rwset"
)

// NewCollRWSetFilter returns a new collection RW set filter
func NewCollRWSetFilter() *CollRWSetFilter {
	return &CollRWSetFilter{}
}

// CollRWSetFilter filters out all off-ledger (including transient data) read-write sets from the simulation results
// so that they won't be included in the block.
type CollRWSetFilter struct {
}

// Initialize initializes the filter
func (f *CollRWSetFilter) Initialize() *CollRWSetFilter {
	// Noop
	return f
}

// Filter is a noop filter. It simply returns the passed in r/w set
func (f *CollRWSetFilter) Filter(channelID string, pubSimulationResults *rwset.TxReadWriteSet) (*rwset.TxReadWriteSet, error) {
	return pubSimulationResults, nil
}
