/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package kvledger

import (
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/kvledger"
)

// UpgradeDBs upgrades existing ledger databases to the latest formats.
// It checks the format of idStore and does not drop any databases
// if the format is already the latest version. Otherwise, it drops
// ledger databases and upgrades the idStore format.
func UpgradeDBs(config *ledger.Config) error {
	return kvledger.UpgradeDBs(config)
}
