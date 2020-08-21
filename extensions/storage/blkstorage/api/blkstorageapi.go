/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blkstorage

import (
	"github.com/hyperledger/fabric-protos-go/common"
)

//BlockStoreExtension is an extension to blkstorage.BlockStore interface which can be used to extend existing block store features.
type BlockStoreExtension interface {
	//CheckpointBlock updates checkpoint info of blockstore with given block
	// and invokes the given notifier before the checkpoint is broadcast
	CheckpointBlock(block *common.Block, notify func()) error
}
