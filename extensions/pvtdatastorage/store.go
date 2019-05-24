/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package pvtdatastorage

import (
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/pvtdatastorage"
)

// NewProvider instantiates a StoreProvider
func NewProvider(conf *ledger.PrivateData) pvtdatastorage.Provider {
	return pvtdatastorage.NewProvider(conf)
}
