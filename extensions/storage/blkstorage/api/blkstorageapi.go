/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blkstorage

import (
	"github.com/hyperledger/fabric/protos/common"
)

//BlockStoreExtension is an extension to blkstorage.BlockStore interface which can be used to extend existing block store features.
type BlockStoreExtension interface {
	//CheckpointBlock updates checkpoint info of blockstore with given block
	CheckpointBlock(block *common.Block) error
}
