/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package ledger

import (
	"github.com/hyperledger/fabric/common/ledger/blkstorage"
	"github.com/hyperledger/fabric/extensions/ledger/api"
	"github.com/hyperledger/fabric/protos/common"
)

//NewKVLedgerExtension returns peer ledger extension implementation using block store provided
func NewKVLedgerExtension(store blkstorage.BlockStore) api.PeerLedgerExtension {
	return &kvLedger{store}
}

//kvLedger is implementation of Peer Ledger extension
type kvLedger struct {
	blockStore blkstorage.BlockStore
}

// CheckpointBlock updates checkpoint info of underlying blockstore with given block
func (l *kvLedger) CheckpointBlock(block *common.Block) error {
	return l.blockStore.CheckpointBlock(block)
}
