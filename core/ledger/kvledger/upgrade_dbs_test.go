/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package kvledger

import (
	"fmt"
	"testing"

	"github.com/hyperledger/fabric/common/ledger/dataformat"
	"github.com/hyperledger/fabric/core/ledger/mock"
	xtestutil "github.com/hyperledger/fabric/extensions/testutil"
	"github.com/stretchr/testify/require"
)

func TestUpgradeWrongFormat(t *testing.T) {
	xtestutil.Skip(t, "This test is only valid for LevelDB ID store")

	conf, cleanup := testConfig(t)
	conf.HistoryDBConfig.Enabled = false
	defer cleanup()
	provider := testutilNewProvider(conf, t, &mock.DeployedChaincodeInfoProvider{})

	// change format to a wrong value to test UpgradeFormat error path
	err := provider.idStore.(*idStore).db.Put(formatKey, []byte("x.0"), true)
	provider.Close()
	require.NoError(t, err)

	err = UpgradeDBs(conf)
	expectedErr := &dataformat.ErrFormatMismatch{
		ExpectedFormat: dataformat.PreviousFormat,
		Format:         "x.0",
		DBInfo:         fmt.Sprintf("leveldb for channel-IDs at [%s]", LedgerProviderPath(conf.RootFSPath)),
	}
	require.EqualError(t, err, expectedErr.Error())
}
