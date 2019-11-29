/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package idstore

import (
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/kvledger/idstore"
	storeapi "github.com/hyperledger/fabric/extensions/storage/api"
)

// OpenIDStore open idstore
func OpenIDStore(path string, ledgerconfig *ledger.Config) (storeapi.IDStore, error) {
	return idstore.OpenIDStore(path)
}
