/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package idstore

import (
	"github.com/hyperledger/fabric/core/ledger"
	storeapi "github.com/hyperledger/fabric/extensions/storage/api"
)

// OpenIDStoreHandler opens an ID store
type OpenIDStoreHandler func(path string, ledgerconfig *ledger.Config) (storeapi.IDStore, error)

// OpenIDStore open idstore
func OpenIDStore(path string, ledgerconfig *ledger.Config, defaultHandler OpenIDStoreHandler) (storeapi.IDStore, error) {
	return defaultHandler(path, ledgerconfig)
}
