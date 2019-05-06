/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package store

import (
	"time"

	proto "github.com/hyperledger/fabric/protos/transientstore"
)

// ExpiringValue is holds the value and expiration time.
type ExpiringValue struct {
	Value  []byte
	Expiry time.Time
}

type ExpiringValues []*ExpiringValue

// Store manages the storage of private data collections.
type Store interface {
	// Persist stores the private write set of a transaction.
	Persist(txid string, privateSimulationResultsWithConfig *proto.TxPvtReadWriteSetWithConfigInfo) error

	// Close closes the store
	Close()
}

// Retriever retrieves private data
type Retriever interface {
}

// Retriever provides private data retrievers
type Provider interface {
	RetrieverForChannel(channel string) Retriever
}
