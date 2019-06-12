/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"github.com/hyperledger/fabric/core/ledger"
	xgossipapi "github.com/hyperledger/fabric/extensions/gossip/api"
)

// QueryExecutorProvider returns a query executor
type QueryExecutorProvider interface {
	NewQueryExecutor() (ledger.QueryExecutor, error)
}

// QueryExecutorProviderFactory returns a query executor provider for a given channel
type QueryExecutorProviderFactory interface {
	GetQueryExecutorProvider(channelID string) QueryExecutorProvider
}

// BlockPublisherProvider returns a block publisher for a given channel
type BlockPublisherProvider interface {
	ForChannel(channelID string) xgossipapi.BlockPublisher
}
