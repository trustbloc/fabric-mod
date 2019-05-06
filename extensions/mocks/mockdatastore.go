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
	err error
}

// NewDataStore returns a mock private data store
func NewDataStore() *DataStore {
	return &DataStore{}
}

// Error sets an err
func (m *DataStore) Error(err error) *DataStore {
	m.err = err
	return m
}

// Persist stores the private write set of a transaction along with the collection config
// in the transient store based on txid and the block height the private data was received at
func (m *DataStore) Persist(txid string, privateSimulationResultsWithConfig *proto.TxPvtReadWriteSetWithConfigInfo) error {
	return m.err
}

// Close closes the store
func (m *DataStore) Close() {
}
