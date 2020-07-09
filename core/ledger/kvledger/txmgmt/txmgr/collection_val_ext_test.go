/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package txmgr

import (
	"testing"

	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/internal/version"
	"github.com/hyperledger/fabric/core/ledger/mock"
	"github.com/stretchr/testify/assert"
)

func TestUnknownCollectionValidation(t *testing.T) {
	testEnv := testEnvsMap[levelDBtestEnvName]
	testEnv.init(t, "testLedger", nil)
	txMgr := testEnv.getTxMgr()

	populateUnknownCollConfigForTest(t, txMgr,
		[]collConfigkey{
			{"ns1", "coll1"},
		},
		version.NewHeight(1, 1),
	)

	sim, err := txMgr.NewTxSimulator("tx-id1")
	assert.NoError(t, err)

	const key1 = "key1"
	const key2 = "key1"

	_, err = sim.GetPrivateData("ns1", "coll1", key1)
	assert.NoError(t, err)

	_, err = sim.GetPrivateDataMultipleKeys("ns1", "coll1", []string{key1, key2})
	assert.NoError(t, err)
}

func populateUnknownCollConfigForTest(t *testing.T, txMgr *LockBasedTxMgr, nsColls []collConfigkey, ht *version.Height) {
	m := map[string]*peer.CollectionConfigPackage{}
	for _, nsColl := range nsColls {
		ns, coll := nsColl.ns, nsColl.coll
		pkg, ok := m[ns]
		if !ok {
			pkg = &peer.CollectionConfigPackage{}
			m[ns] = pkg
		}
		sCollConfig := &peer.CollectionConfig_StaticCollectionConfig{
			StaticCollectionConfig: &peer.StaticCollectionConfig{
				Name: coll,
				Type: peer.CollectionType_COL_UNKNOWN,
			},
		}
		pkg.Config = append(pkg.Config, &peer.CollectionConfig{Payload: sCollConfig})
	}
	ccInfoProvider := &mock.DeployedChaincodeInfoProvider{}
	ccInfoProvider.AllCollectionsConfigPkgStub = func(channelName, ccName string, qe ledger.SimpleQueryExecutor) (*peer.CollectionConfigPackage, error) {
		return m[ccName], nil
	}
	txMgr.ccInfoProvider = ccInfoProvider
}
