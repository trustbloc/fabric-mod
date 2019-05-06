/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package mocks

import (
	proto "github.com/hyperledger/fabric/protos/transientstore"
)

// DataStore implements a mock private data store
type DataStore struct {
}

// NewDataStore returns a mock private data store
func NewDataStore() *DataStore {
	return &DataStore{}
}

// Persist stores the private write set of a transaction along with the collection config
// in the transient store based on txid and the block height the private data was received at
func (m *DataStore) Persist(txid string, privateSimulationResultsWithConfig *proto.TxPvtReadWriteSetWithConfigInfo) error {
	return nil
}

// Close closes the store
func (m *DataStore) Close() {
}
