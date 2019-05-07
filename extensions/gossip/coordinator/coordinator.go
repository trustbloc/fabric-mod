/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package coordinator

import (
	storeapi "github.com/hyperledger/fabric/extensions/collections/api/store"
	"github.com/hyperledger/fabric/protos/transientstore"
)

type transientStore interface {
	// PersistWithConfig stores the private write set of a transaction along with the collection config
	// in the transient store based on txid and the block height the private data was received at
	PersistWithConfig(txid string, blockHeight uint64, privateSimulationResultsWithConfig *transientstore.TxPvtReadWriteSetWithConfigInfo) error
}

// Coordinator is the extensions Gossip coordinator
type Coordinator struct {
	transientStore transientStore
}

// New returns a new Coordinator
func New(channelID string, transientStore transientStore, collDataStore storeapi.Store) *Coordinator {
	return &Coordinator{
		transientStore: transientStore,
	}
}

// StorePvtData used to persist private date into transient store
func (c *Coordinator) StorePvtData(txID string, privData *transientstore.TxPvtReadWriteSetWithConfigInfo, blkHeight uint64) error {
	return c.transientStore.PersistWithConfig(txID, blkHeight, privData)
}
