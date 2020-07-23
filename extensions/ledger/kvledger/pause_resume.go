/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package kvledger

import (
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/kvledger"
)

// PauseChannel updates the channel status to inactive in ledgerProviders.
func PauseChannel(config *ledger.Config, ledgerID string) error {
	return kvledger.PauseChannel(config, ledgerID)
}

// ResumeChannel updates the channel status to active in ledgerProviders
func ResumeChannel(config *ledger.Config, ledgerID string) error {
	return kvledger.ResumeChannel(config, ledgerID)
}
