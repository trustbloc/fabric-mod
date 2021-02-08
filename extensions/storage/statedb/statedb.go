/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package statedb

import (
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/statedb"
	gossipapi "github.com/hyperledger/fabric/extensions/gossip/api"
)

// QueryExecutorProvider provides a query executor with and without a commit lock
type QueryExecutorProvider interface {
	NewQueryExecutor() (ledger.QueryExecutor, error)
	NewQueryExecutorNoLock() (ledger.QueryExecutor, error)
}

//AddCCUpgradeHandler adds chaincode upgrade handler to blockpublisher
func AddCCUpgradeHandler(chainName string, handler gossipapi.ChaincodeUpgradeHandler) {
	//do nothing
}

// Register registers a state database for a given channel
func Register(channelID string, db statedb.VersionedDB, qep QueryExecutorProvider) {
	// do nothing
}
