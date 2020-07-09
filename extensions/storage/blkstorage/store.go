/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blkstorage

import (
	"github.com/hyperledger/fabric/common/ledger/blkstorage"
	"github.com/hyperledger/fabric/common/metrics"
	"github.com/hyperledger/fabric/core/ledger"
	extledgerapi "github.com/hyperledger/fabric/extensions/ledger/api"
)

//NewProvider is redirect hook for fabric/fsblkstorage NewProvider()
func NewProvider(conf *blkstorage.Conf, indexConfig *blkstorage.IndexConfig, _ *ledger.Config, metricsProvider metrics.Provider) (extledgerapi.BlockStoreProvider, error) {
	return blkstorage.NewProvider(conf, indexConfig, metricsProvider)
}

//NewConf is redirect hook for fabric/fsblkstorage NewConf()
func NewConf(blockStorageDir string, maxBlockfileSize int) *blkstorage.Conf {
	return blkstorage.NewConf(blockStorageDir, maxBlockfileSize)
}
