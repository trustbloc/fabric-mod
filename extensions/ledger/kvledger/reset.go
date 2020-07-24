/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package kvledger

import (
	"github.com/hyperledger/fabric/common/ledger/blkstorage"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/kvledger"
)

func LoadPreResetHeight(ledgerconfig *ledger.Config, ledgerIDs []string) (map[string]uint64, error) {
	return kvledger.LoadPreResetHeight(ledgerconfig.RootFSPath, ledgerIDs)
}

func ClearPreResetHeight(ledgerconfig *ledger.Config, ledgerIDs []string) error {
	return kvledger.ClearPreResetHeight(ledgerconfig.RootFSPath, ledgerIDs)
}

func ResetAllKVLedgers(ledgerconfig *ledger.Config) error {
	return kvledger.ResetAllKVLedgers(ledgerconfig.RootFSPath)
}

func ResetBlockStore(ledgerconfig *ledger.Config) error {
	return blkstorage.ResetBlockStore(kvledger.BlockStorePath(ledgerconfig.RootFSPath))
}
