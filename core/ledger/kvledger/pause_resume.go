/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package kvledger

import (
	"github.com/hyperledger/fabric/common/ledger/util/leveldbhelper"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/kvledger/msgs"
	xstorageapi "github.com/hyperledger/fabric/extensions/storage/api"
	xidstore "github.com/hyperledger/fabric/extensions/storage/idstore"
	"github.com/pkg/errors"
)

// PauseChannel updates the channel status to inactive in ledgerProviders.
func PauseChannel(config *ledger.Config, ledgerID string) error {
	if err := pauseOrResumeChannel(config, ledgerID, msgs.Status_INACTIVE); err != nil {
		return err
	}
	logger.Infof("The channel [%s] has been successfully paused", ledgerID)
	return nil
}

// ResumeChannel updates the channel status to active in ledgerProviders
func ResumeChannel(config *ledger.Config, ledgerID string) error {
	if err := pauseOrResumeChannel(config, ledgerID, msgs.Status_ACTIVE); err != nil {
		return err
	}
	logger.Infof("The channel [%s] has been successfully resumed", ledgerID)
	return nil
}

func pauseOrResumeChannel(config *ledger.Config, ledgerID string, status msgs.Status) error {
	fileLock := leveldbhelper.NewFileLock(fileLockPath(config.RootFSPath))
	if err := fileLock.Lock(); err != nil {
		return errors.Wrap(err, "as another peer node command is executing,"+
			" wait for that command to complete its execution or terminate it before retrying")
	}
	defer fileLock.Unlock()

	idStore, err := xidstore.OpenIDStore(LedgerProviderPath(config.RootFSPath), config,
		func(path string, _ *ledger.Config) (xstorageapi.IDStore, error) {
			return openIDStore(path)
		},
	)
	if err != nil {
		return err
	}
	defer idStore.Close()
	return idStore.UpdateLedgerStatus(ledgerID, status)
}
