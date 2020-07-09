/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package pvtdatastorage

import (
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/pvtdatastorage"
	xstorageapi "github.com/hyperledger/fabric/extensions/storage/api"
)

// NewProvider instantiates a StoreProvider
func NewProvider(conf *pvtdatastorage.PrivateDataConfig, ledgerconfig *ledger.Config) (xstorageapi.PrivateDataProvider, error) {
	return pvtdatastorage.NewProvider(conf)
}
