/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package validation

import (
	"context"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/bccsp"
	"github.com/hyperledger/fabric/common/channelconfig"
	"github.com/hyperledger/fabric/common/policies"
	"github.com/hyperledger/fabric/core/committer/txvalidator"
	"github.com/hyperledger/fabric/core/committer/txvalidator/plugin"
	validatorv14 "github.com/hyperledger/fabric/core/committer/txvalidator/v14"
	validatorv20 "github.com/hyperledger/fabric/core/committer/txvalidator/v20"
	"github.com/hyperledger/fabric/core/committer/txvalidator/v20/plugindispatcher"
	"github.com/hyperledger/fabric/msp"

	"github.com/hyperledger/fabric/core/ledger"
)

//go:generate counterfeiter -o mocks/channelresources.go --fake-name ChannelResources . ChannelResources

type semaphore interface {
	Acquire(ctx context.Context) error
	Release()
}

type channelResources interface {
	MSPManager() msp.MSPManager
	Apply(configtx *common.ConfigEnvelope) error
	GetMSPIDs() []string
	Capabilities() channelconfig.ApplicationCapabilities
	Ledger() ledger.PeerLedger
}

type ledgerResources interface {
	GetTransactionByID(txID string) (*peer.ProcessedTransaction, error)
	NewQueryExecutor() (ledger.QueryExecutor, error)
}

// NewTxValidator creates a new transaction validator
func NewTxValidator(
	channelID string,
	sem semaphore,
	cr channelResources,
	ler ledgerResources,
	lcr plugindispatcher.LifecycleResources,
	cor plugindispatcher.CollectionResources,
	pm plugin.Mapper,
	channelPolicyManagerGetter policies.ChannelPolicyManagerGetter,
	cryptoProvider bccsp.BCCSP,
) txvalidator.Validator {
	return &txvalidator.ValidationRouter{
		CapabilityProvider: cr,
		V14Validator:       validatorv14.NewTxValidator(channelID, sem, cr, pm, cryptoProvider),
		V20Validator:       validatorv20.NewTxValidator(channelID, sem, cr, ler, lcr, cor, pm, channelPolicyManagerGetter, cryptoProvider),
	}
}
