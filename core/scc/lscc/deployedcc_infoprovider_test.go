/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package lscc_test

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/core/common/ccprovider"
	"github.com/hyperledger/fabric/core/common/privdata"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/scc/lscc"
	"github.com/hyperledger/fabric/core/scc/lscc/mock"
	"github.com/stretchr/testify/assert"
)

func TestNamespaces(t *testing.T) {
	ccInfoProvdier := &lscc.DeployedCCInfoProvider{}
	namespaces := ccInfoProvdier.Namespaces()
	assert.Len(t, namespaces, 1)
	assert.Equal(t, "lscc", namespaces[0])
}

func TestChaincodeInfo(t *testing.T) {
	cc1 := &ledger.DeployedChaincodeInfo{
		Name:    "cc1",
		Version: "cc1_version",
		Hash:    []byte("cc1_hash"),
	}

	cc2 := &ledger.DeployedChaincodeInfo{
		Name:                        "cc2",
		Version:                     "cc2_version",
		Hash:                        []byte("cc2_hash"),
		ExplicitCollectionConfigPkg: prepapreCollectionConfigPkg([]string{"cc2_coll1", "cc2_coll2"}),
	}

	mockQE := prepareMockQE(t, []*ledger.DeployedChaincodeInfo{cc1, cc2})
	ccInfoProvdier := &lscc.DeployedCCInfoProvider{}

	ccInfo1, err := ccInfoProvdier.ChaincodeInfo("", "cc1", mockQE)
	assert.NoError(t, err)
	expectedCCInfo1 := cc1
	expectedCCInfo1.IsLegacy = true
	assert.Equal(t, expectedCCInfo1, ccInfo1)

	ccInfo2, err := ccInfoProvdier.ChaincodeInfo("", "cc2", mockQE)
	assert.NoError(t, err)
	assert.Equal(t, cc2.Name, ccInfo2.Name)
	assert.True(t, proto.Equal(cc2.ExplicitCollectionConfigPkg, ccInfo2.ExplicitCollectionConfigPkg))

	ccInfo3, err := ccInfoProvdier.ChaincodeInfo("", "cc3", mockQE)
	assert.NoError(t, err)
	assert.Nil(t, ccInfo3)
}

func TestCollectionInfo(t *testing.T) {
	cc1 := &ledger.DeployedChaincodeInfo{
		Name:    "cc1",
		Version: "cc1_version",
		Hash:    []byte("cc1_hash"),
	}

	cc2 := &ledger.DeployedChaincodeInfo{
		Name:                        "cc2",
		Version:                     "cc2_version",
		Hash:                        []byte("cc2_hash"),
		ExplicitCollectionConfigPkg: prepapreCollectionConfigPkg([]string{"cc2_coll1", "cc2_coll2"}),
	}

	mockQE := prepareMockQE(t, []*ledger.DeployedChaincodeInfo{cc1, cc2})
	ccInfoProvdier := &lscc.DeployedCCInfoProvider{}

	collInfo1, err := ccInfoProvdier.CollectionInfo("", "cc1", "non-existing-coll-in-cc1", mockQE)
	assert.NoError(t, err)
	assert.Nil(t, collInfo1)

	collInfo2, err := ccInfoProvdier.CollectionInfo("", "cc2", "cc2_coll1", mockQE)
	assert.NoError(t, err)
	assert.Equal(t, "cc2_coll1", collInfo2.Name)

	collInfo3, err := ccInfoProvdier.CollectionInfo("", "cc2", "non-existing-coll-in-cc2", mockQE)
	assert.NoError(t, err)
	assert.Nil(t, collInfo3)

	ccPkg1, err := ccInfoProvdier.AllCollectionsConfigPkg("", "cc1", mockQE)
	assert.NoError(t, err)
	assert.Nil(t, ccPkg1)

	ccPkg2, err := ccInfoProvdier.AllCollectionsConfigPkg("", "cc2", mockQE)
	assert.NoError(t, err)
	assert.Equal(t, prepapreCollectionConfigPkg([]string{"cc2_coll1", "cc2_coll2"}), ccPkg2)
}

func prepareMockQE(t *testing.T, deployedChaincodes []*ledger.DeployedChaincodeInfo) *mock.QueryExecutor {
	mockQE := &mock.QueryExecutor{}
	lsccTable := map[string][]byte{}
	for _, cc := range deployedChaincodes {
		chaincodeData := &ccprovider.ChaincodeData{Name: cc.Name, Version: cc.Version, Id: cc.Hash}
		chaincodeDataBytes, err := proto.Marshal(chaincodeData)
		assert.NoError(t, err)
		lsccTable[cc.Name] = chaincodeDataBytes

		if cc.ExplicitCollectionConfigPkg != nil {
			collConfigPkgByte, err := proto.Marshal(cc.ExplicitCollectionConfigPkg)
			assert.NoError(t, err)
			lsccTable[privdata.BuildCollectionKVSKey(cc.Name)] = collConfigPkgByte
		}
	}

	mockQE.GetStateStub = func(ns, key string) ([]byte, error) {
		return lsccTable[key], nil
	}
	return mockQE
}

func prepapreCollectionConfigPkg(collNames []string) *peer.CollectionConfigPackage {
	pkg := &peer.CollectionConfigPackage{}
	for _, collName := range collNames {
		sCollConfig := &peer.CollectionConfig_StaticCollectionConfig{
			StaticCollectionConfig: &peer.StaticCollectionConfig{
				Name: collName,
			},
		}
		config := &peer.CollectionConfig{Payload: sCollConfig}
		pkg.Config = append(pkg.Config, config)
	}
	return pkg
}
