/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blkstorage

import (
	"github.com/hyperledger/fabric/common/ledger/blkstorage"
	"github.com/hyperledger/fabric/common/ledger/blkstorage/fsblkstorage"
	"github.com/hyperledger/fabric/common/metrics"
	"github.com/hyperledger/fabric/core/ledger"
)

//NewProvider is redirect hook for fabric/fsblkstorage NewProvider()
func NewProvider(conf *fsblkstorage.Conf, indexConfig *blkstorage.IndexConfig, ledgerconfig *ledger.Config, metricsProvider metrics.Provider) (blkstorage.BlockStoreProvider, error) {
	return fsblkstorage.NewProvider(conf, indexConfig, metricsProvider)
}

//NewConf is redirect hook for fabric/fsblkstorage NewConf()
func NewConf(blockStorageDir string, maxBlockfileSize int) *fsblkstorage.Conf {
	return fsblkstorage.NewConf(blockStorageDir, maxBlockfileSize)
}
