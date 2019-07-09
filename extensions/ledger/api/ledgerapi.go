/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import "github.com/hyperledger/fabric/protos/common"

//PeerLedgerExtension is an extension to PeerLedger interface which can be used to extend existing peer ledger features.
type PeerLedgerExtension interface {
	//CheckpointBlock updates check point info in underlying store
	CheckpointBlock(block *common.Block) error
}
