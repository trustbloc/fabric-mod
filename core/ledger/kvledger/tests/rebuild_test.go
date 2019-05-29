/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package tests

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperledger/fabric/common/ledger/util"
)

func TestRebuildComponents(t *testing.T) {
	env := newEnv(t)
	defer env.cleanup()
	env.initLedgerMgmt()

	h1, h2 := newTestHelperCreateLgr("ledger1", t), newTestHelperCreateLgr("ledger2", t)
	dataHelper := newSampleDataHelper(t)

	dataHelper.populateLedger(h1)
	dataHelper.populateLedger(h2)

	dataHelper.verifyLedgerContent(h1)
	dataHelper.verifyLedgerContent(h2)

	t.Run("rebuild only statedb",
		func(t *testing.T) {
			env.closeAllLedgersAndDrop(rebuildableStatedb)
			h1, h2 := newTestHelperOpenLgr("ledger1", t), newTestHelperOpenLgr("ledger2", t)
			dataHelper.verifyLedgerContent(h1)
			dataHelper.verifyLedgerContent(h2)
		},
	)

	t.Run("rebuild statedb and config history",
		func(t *testing.T) {
			env.closeAllLedgersAndDrop(rebuildableStatedb + rebuildableConfigHistory)
			h1, h2 := newTestHelperOpenLgr("ledger1", t), newTestHelperOpenLgr("ledger2", t)
			dataHelper.verifyLedgerContent(h1)
			dataHelper.verifyLedgerContent(h2)
		},
	)

	t.Run("rebuild statedb and block index",
		func(t *testing.T) {

			flag := rebuildableStatedb + rebuildableBlockIndex
			_, err := util.DirEmpty(filepath.Join(env.rootPath, "ledgersData", "chains"))
			if err != nil && strings.Contains(err.Error(), "no such file or directory") {
				//couch db index, no need to attempt to delete index file
				flag = rebuildableStatedb
			}

			env.closeAllLedgersAndDrop(flag)
			h1, h2 := newTestHelperOpenLgr("ledger1", t), newTestHelperOpenLgr("ledger2", t)
			dataHelper.verifyLedgerContent(h1)
			dataHelper.verifyLedgerContent(h2)
		},
	)
}
