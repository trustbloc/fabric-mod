/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package valimpl

import (
	"fmt"
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/ledger/rwset"
	"github.com/hyperledger/fabric-protos-go/ledger/rwset/kvrwset"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/bccsp/sw"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/common/flogging/floggingtest"
	"github.com/hyperledger/fabric/common/ledger/testutil"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/privacyenabledstate"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/rwsetutil"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/statedb"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/txmgr"
	mocktxmgr "github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/txmgr/mock"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/validator/internal"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/validator/valimpl/mock"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/version"
	mocklgr "github.com/hyperledger/fabric/core/ledger/mock"
	lutils "github.com/hyperledger/fabric/core/ledger/util"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	flogging.ActivateSpec("valimpl,statebasedval,internal=debug")
	os.Exit(m.Run())
}

func TestValidateAndPreparePvtBatch(t *testing.T) {
	testDBEnv := &privacyenabledstate.LevelDBCommonStorageTestEnv{}
	testDBEnv.Init(t)
	defer testDBEnv.Cleanup()
	testDB := testDBEnv.GetDBHandle("emptydb")

	pubSimulationResults := [][]byte{}
	pvtDataMap := make(map[uint64]*ledger.TxPvtData)

	txids := []string{"tx1", "tx2", "tx3"}

	// 1. Construct a block with three transactions and pre
	//    process the block by calling preprocessProtoBlock()
	//    and get a preprocessedBlock.

	// Tx 1
	// Get simulation results for tx1
	tx1SimulationResults := testutilSampleTxSimulationResults(t, "key1")
	res, err := tx1SimulationResults.GetPubSimulationBytes()
	assert.NoError(t, err)

	// Add tx1 public rwset to the set of results
	pubSimulationResults = append(pubSimulationResults, res)

	// Add tx1 private rwset to the private data map
	tx1PvtData := &ledger.TxPvtData{SeqInBlock: 0, WriteSet: tx1SimulationResults.PvtSimulationResults}
	pvtDataMap[uint64(0)] = tx1PvtData

	// Tx 2
	// Get simulation results for tx2
	tx2SimulationResults := testutilSampleTxSimulationResults(t, "key2")
	res, err = tx2SimulationResults.GetPubSimulationBytes()
	assert.NoError(t, err)

	// Add tx2 public rwset to the set of results
	pubSimulationResults = append(pubSimulationResults, res)

	// As tx2 private rwset does not belong to a collection owned by the current peer,
	// the private rwset is not added to the private data map

	// Tx 3
	// Get simulation results for tx3
	tx3SimulationResults := testutilSampleTxSimulationResults(t, "key3")
	res, err = tx3SimulationResults.GetPubSimulationBytes()
	assert.NoError(t, err)

	// Add tx3 public rwset to the set of results
	pubSimulationResults = append(pubSimulationResults, res)

	// Add tx3 private rwset to the private data map
	tx3PvtData := &ledger.TxPvtData{SeqInBlock: 2, WriteSet: tx3SimulationResults.PvtSimulationResults}
	pvtDataMap[uint64(2)] = tx3PvtData

	// Construct a block using all three transactions' simulation results
	block := testutil.ConstructBlockWithTxid(t, 10, testutil.ConstructRandomBytes(t, 32), pubSimulationResults, txids, false)

	// Construct the expected preprocessed block from preprocessProtoBlock()
	expectedPerProcessedBlock := &internal.Block{Num: 10}
	tx1TxRWSet, err := rwsetutil.TxRwSetFromProtoMsg(tx1SimulationResults.PubSimulationResults)
	assert.NoError(t, err)
	expectedPerProcessedBlock.Txs = append(expectedPerProcessedBlock.Txs, &internal.Transaction{IndexInBlock: 0, ID: "tx1", RWSet: tx1TxRWSet})

	tx2TxRWSet, err := rwsetutil.TxRwSetFromProtoMsg(tx2SimulationResults.PubSimulationResults)
	assert.NoError(t, err)
	expectedPerProcessedBlock.Txs = append(expectedPerProcessedBlock.Txs, &internal.Transaction{IndexInBlock: 1, ID: "tx2", RWSet: tx2TxRWSet})

	tx3TxRWSet, err := rwsetutil.TxRwSetFromProtoMsg(tx3SimulationResults.PubSimulationResults)
	assert.NoError(t, err)
	expectedPerProcessedBlock.Txs = append(expectedPerProcessedBlock.Txs, &internal.Transaction{IndexInBlock: 2, ID: "tx3", RWSet: tx3TxRWSet})
	alwaysValidKVFunc := func(key string, value []byte) error {
		return nil
	}
	actualPreProcessedBlock, _, err := preprocessProtoBlock(nil, alwaysValidKVFunc, block, false, nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedPerProcessedBlock, actualPreProcessedBlock)

	// 2. Assuming that MVCC validation is performed on the preprocessedBlock, set the appropriate validation code
	//    for each transaction and then call validateAndPreparePvtBatch() to get a validated private update batch.
	//    Here, validate refers to comparison of hash of pvtRWSet in public rwset with the actual hash of pvtRWSet)

	// Set validation code for all three transactions. One of the three transaction is marked invalid
	mvccValidatedBlock := actualPreProcessedBlock
	mvccValidatedBlock.Txs[0].ValidationCode = peer.TxValidationCode_VALID
	mvccValidatedBlock.Txs[1].ValidationCode = peer.TxValidationCode_VALID
	mvccValidatedBlock.Txs[2].ValidationCode = peer.TxValidationCode_INVALID_OTHER_REASON

	// Construct the expected private updates
	expectedPvtUpdates := privacyenabledstate.NewPvtUpdateBatch()
	tx1TxPvtRWSet, err := rwsetutil.TxPvtRwSetFromProtoMsg(tx1SimulationResults.PvtSimulationResults)
	assert.NoError(t, err)
	addPvtRWSetToPvtUpdateBatch(tx1TxPvtRWSet, expectedPvtUpdates, version.NewHeight(uint64(10), uint64(0)))

	actualPvtUpdates, err := validateAndPreparePvtBatch(mvccValidatedBlock, testDB, nil, pvtDataMap, nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedPvtUpdates, actualPvtUpdates)

	expectedtxsFilter := []uint8{uint8(peer.TxValidationCode_VALID), uint8(peer.TxValidationCode_VALID), uint8(peer.TxValidationCode_INVALID_OTHER_REASON)}

	postprocessProtoBlock(block, mvccValidatedBlock)
	assert.Equal(t, expectedtxsFilter, block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER])
}

func TestPreprocessProtoBlock(t *testing.T) {
	allwaysValidKVfunc := func(key string, value []byte) error {
		return nil
	}
	// good block
	//_, gb := testutil.NewBlockGenerator(t, "testLedger", false)
	gb := testutil.ConstructTestBlock(t, 10, 1, 1)
	_, _, err := preprocessProtoBlock(nil, allwaysValidKVfunc, gb, false, nil)
	assert.NoError(t, err)
	// bad envelope
	gb = testutil.ConstructTestBlock(t, 11, 1, 1)
	gb.Data = &common.BlockData{Data: [][]byte{{123}}}
	gb.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER] =
		lutils.NewTxValidationFlagsSetValue(len(gb.Data.Data), peer.TxValidationCode_VALID)
	_, _, err = preprocessProtoBlock(nil, allwaysValidKVfunc, gb, false, nil)
	assert.Error(t, err)
	t.Log(err)
	// bad payload
	gb = testutil.ConstructTestBlock(t, 12, 1, 1)
	envBytes, _ := protoutil.GetBytesEnvelope(&common.Envelope{Payload: []byte{123}})
	gb.Data = &common.BlockData{Data: [][]byte{envBytes}}
	_, _, err = preprocessProtoBlock(nil, allwaysValidKVfunc, gb, false, nil)
	assert.Error(t, err)
	t.Log(err)
	// bad channel header
	gb = testutil.ConstructTestBlock(t, 13, 1, 1)
	payloadBytes, _ := protoutil.GetBytesPayload(&common.Payload{
		Header: &common.Header{ChannelHeader: []byte{123}},
	})
	envBytes, _ = protoutil.GetBytesEnvelope(&common.Envelope{Payload: payloadBytes})
	gb.Data = &common.BlockData{Data: [][]byte{envBytes}}
	_, _, err = preprocessProtoBlock(nil, allwaysValidKVfunc, gb, false, nil)
	assert.Error(t, err)
	t.Log(err)

	// bad channel header with invalid filter set
	gb = testutil.ConstructTestBlock(t, 14, 1, 1)
	payloadBytes, _ = protoutil.GetBytesPayload(&common.Payload{
		Header: &common.Header{ChannelHeader: []byte{123}},
	})
	envBytes, _ = protoutil.GetBytesEnvelope(&common.Envelope{Payload: payloadBytes})
	gb.Data = &common.BlockData{Data: [][]byte{envBytes}}
	flags := lutils.NewTxValidationFlags(len(gb.Data.Data))
	flags.SetFlag(0, peer.TxValidationCode_BAD_CHANNEL_HEADER)
	gb.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER] = flags
	_, _, err = preprocessProtoBlock(nil, allwaysValidKVfunc, gb, false, nil)
	assert.NoError(t, err) // invalid filter should take precedence

	// new block
	var blockNum uint64 = 15
	txid := "testtxid1234"
	gb = testutil.ConstructBlockWithTxid(t, blockNum, []byte{123},
		[][]byte{{123}}, []string{txid}, false)
	flags = lutils.NewTxValidationFlags(len(gb.Data.Data))
	flags.SetFlag(0, peer.TxValidationCode_BAD_HEADER_EXTENSION)
	gb.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER] = flags

	// test logger
	oldLogger := logger
	defer func() { logger = oldLogger }()
	l, recorder := floggingtest.NewTestLogger(t)
	logger = l

	_, _, err = preprocessProtoBlock(nil, allwaysValidKVfunc, gb, false, nil)
	assert.NoError(t, err)
	expected := fmt.Sprintf(
		"Channel [%s]: Block [%d] Transaction index [%d] TxId [%s] marked as invalid by committer. Reason code [%s]",
		"testchannelid", blockNum, 0, txid, peer.TxValidationCode_BAD_HEADER_EXTENSION,
	)
	assert.NotEmpty(t, recorder.MessagesContaining(expected))
}

func TestPreprocessProtoBlockInvalidWriteset(t *testing.T) {
	kvValidationFunc := func(key string, value []byte) error {
		if value[0] == '_' {
			return fmt.Errorf("value [%s] found to be invalid by 'kvValidationFunc for testing'", value)
		}
		return nil
	}

	rwSetBuilder := rwsetutil.NewRWSetBuilder()
	rwSetBuilder.AddToWriteSet("ns", "key", []byte("_invalidValue")) // bad value
	simulation1, err := rwSetBuilder.GetTxSimulationResults()
	assert.NoError(t, err)
	simulation1Bytes, err := simulation1.GetPubSimulationBytes()
	assert.NoError(t, err)

	rwSetBuilder = rwsetutil.NewRWSetBuilder()
	rwSetBuilder.AddToWriteSet("ns", "key", []byte("validValue")) // good value
	simulation2, err := rwSetBuilder.GetTxSimulationResults()
	assert.NoError(t, err)
	simulation2Bytes, err := simulation2.GetPubSimulationBytes()
	assert.NoError(t, err)

	block := testutil.ConstructBlock(t, 1, testutil.ConstructRandomBytes(t, 32),
		[][]byte{simulation1Bytes, simulation2Bytes}, false) // block with two txs
	txfilter := lutils.TxValidationFlags(block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER])
	assert.True(t, txfilter.IsValid(0))
	assert.True(t, txfilter.IsValid(1)) // both txs are valid initially at the time of block cutting

	internalBlock, _, err := preprocessProtoBlock(nil, kvValidationFunc, block, false, nil)
	assert.NoError(t, err)
	assert.False(t, txfilter.IsValid(0)) // tx at index 0 should be marked as invalid
	assert.True(t, txfilter.IsValid(1))  // tx at index 1 should be marked as valid
	assert.Len(t, internalBlock.Txs, 1)
	assert.Equal(t, internalBlock.Txs[0].IndexInBlock, 1)
}

func TestIncrementPvtdataVersionIfNeeded(t *testing.T) {
	testDBEnv := &privacyenabledstate.LevelDBCommonStorageTestEnv{}
	testDBEnv.Init(t)
	defer testDBEnv.Cleanup()
	testDB := testDBEnv.GetDBHandle("testdb")
	updateBatch := privacyenabledstate.NewUpdateBatch()
	// populate db with some pvt data
	updateBatch.PvtUpdates.Put("ns", "coll1", "key1", []byte("value1"), version.NewHeight(1, 1))
	updateBatch.PvtUpdates.Put("ns", "coll2", "key2", []byte("value2"), version.NewHeight(1, 2))
	updateBatch.PvtUpdates.Put("ns", "coll3", "key3", []byte("value3"), version.NewHeight(1, 3))
	updateBatch.PvtUpdates.Put("ns", "col4", "key4", []byte("value4"), version.NewHeight(1, 4))
	testDB.ApplyPrivacyAwareUpdates(updateBatch, version.NewHeight(1, 4))

	// for the current block, mimic the resultant hashed updates
	hashUpdates := privacyenabledstate.NewHashedUpdateBatch()
	hashUpdates.PutValHashAndMetadata("ns", "coll1", lutils.ComputeStringHash("key1"),
		lutils.ComputeStringHash("value1_set_by_tx1"), []byte("metadata1_set_by_tx2"), version.NewHeight(2, 2)) // mimics the situation - value set by tx1 and metadata by tx2
	hashUpdates.PutValHashAndMetadata("ns", "coll2", lutils.ComputeStringHash("key2"),
		lutils.ComputeStringHash("value2"), []byte("metadata2_set_by_tx4"), version.NewHeight(2, 4)) // only metadata set by tx4
	hashUpdates.PutValHashAndMetadata("ns", "coll3", lutils.ComputeStringHash("key3"),
		lutils.ComputeStringHash("value3_set_by_tx6"), []byte("metadata3"), version.NewHeight(2, 6)) // only value set by tx6
	pubAndHashedUpdatesBatch := &internal.PubAndHashUpdates{HashUpdates: hashUpdates}

	// for the current block, mimic the resultant pvt updates (without metadata taking into account). Assume that Tx6 pvt data is missing
	pvtUpdateBatch := privacyenabledstate.NewPvtUpdateBatch()
	pvtUpdateBatch.Put("ns", "coll1", "key1", []byte("value1_set_by_tx1"), version.NewHeight(2, 1))
	pvtUpdateBatch.Put("ns", "coll3", "key3", []byte("value3_set_by_tx5"), version.NewHeight(2, 5))
	// metadata updated for key1 and key3
	metadataUpdates := metadataUpdates{collKey{"ns", "coll1", "key1"}: true, collKey{"ns", "coll2", "key2"}: true}

	// invoke function and test results
	err := incrementPvtdataVersionIfNeeded(metadataUpdates, pvtUpdateBatch, pubAndHashedUpdatesBatch, testDB)
	assert.NoError(t, err)

	assert.Equal(t,
		&statedb.VersionedValue{Value: []byte("value1_set_by_tx1"), Version: version.NewHeight(2, 2)}, // key1 value should be same and version should be upgraded to (2,2)
		pvtUpdateBatch.Get("ns", "coll1", "key1"),
	)

	assert.Equal(t,
		&statedb.VersionedValue{Value: []byte("value2"), Version: version.NewHeight(2, 4)}, // key2 entry should get added with value in the db and version (2,4)
		pvtUpdateBatch.Get("ns", "coll2", "key2"),
	)

	assert.Equal(t,
		&statedb.VersionedValue{Value: []byte("value3_set_by_tx5"), Version: version.NewHeight(2, 5)}, // key3 should be unaffected because the tx6 was missing from pvt data
		pvtUpdateBatch.Get("ns", "coll3", "key3"),
	)
}

func TestTxStatsInfoWithConfigTx(t *testing.T) {
	testDBEnv := &privacyenabledstate.LevelDBCommonStorageTestEnv{}
	testDBEnv.Init(t)
	defer testDBEnv.Cleanup()
	testDB := testDBEnv.GetDBHandle("emptydb")

	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	v := NewStatebasedValidator(nil, testDB, nil, cryptoProvider)

	gb := testutil.ConstructTestBlocks(t, 1)[0]
	_, txStatsInfo, err := v.ValidateAndPrepareBatch(&ledger.BlockAndPvtData{Block: gb}, true)
	assert.NoError(t, err)
	expectedTxStatInfo := []*txmgr.TxStatInfo{
		{
			TxType:         common.HeaderType_CONFIG,
			ValidationCode: peer.TxValidationCode_VALID,
		},
	}
	t.Logf("txStatsInfo=%s\n", spew.Sdump(txStatsInfo))
	assert.Equal(t, expectedTxStatInfo, txStatsInfo)
}

func TestContainsPostOrderWrites(t *testing.T) {
	testDBEnv := &privacyenabledstate.LevelDBCommonStorageTestEnv{}
	testDBEnv.Init(t)
	defer testDBEnv.Cleanup()
	testDB := testDBEnv.GetDBHandle("emptydb")
	mockSimulator := &mocklgr.TxSimulator{}
	mockTxmgr := &mocktxmgr.TxMgr{}
	mockTxmgr.NewTxSimulatorReturns(mockSimulator, nil)

	fakeTxProcessor := &mock.Processor{}
	customTxProcessors := map[common.HeaderType]ledger.CustomTxProcessor{
		common.HeaderType_CONFIG: fakeTxProcessor,
	}

	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	v := NewStatebasedValidator(mockTxmgr, testDB, customTxProcessors, cryptoProvider)
	blocks := testutil.ConstructTestBlocks(t, 2)

	// block with config tx that produces post order writes
	fakeTxProcessor.GenerateSimulationResultsStub =
		func(txEnvelop *common.Envelope, s ledger.TxSimulator, initializingLedger bool) error {
			rwSetBuilder := rwsetutil.NewRWSetBuilder()
			rwSetBuilder.AddToWriteSet("ns1", "key1", []byte("value1"))
			rwSetBuilder.GetTxSimulationResults()
			s.(*mocklgr.TxSimulator).GetTxSimulationResultsReturns(
				rwSetBuilder.GetTxSimulationResults())
			return nil
		}
	batch, _, err := v.ValidateAndPrepareBatch(&ledger.BlockAndPvtData{Block: blocks[0]}, true)
	assert.NoError(t, err)
	assert.True(t, batch.PubUpdates.ContainsPostOrderWrites)

	// block with endorser txs
	batch, _, err = v.ValidateAndPrepareBatch(&ledger.BlockAndPvtData{Block: blocks[1]}, true)
	assert.NoError(t, err)
	assert.False(t, batch.PubUpdates.ContainsPostOrderWrites)

	// test with block with invalid config tx
	fakeTxProcessor.GenerateSimulationResultsStub =
		func(txEnvelop *common.Envelope, s ledger.TxSimulator, initializingLedger bool) error {
			s.(*mocklgr.TxSimulator).GetTxSimulationResultsReturns(nil, nil)
			return &ledger.InvalidTxError{Msg: "fake-message"}
		}
	batch, _, err = v.ValidateAndPrepareBatch(&ledger.BlockAndPvtData{Block: blocks[0]}, true)
	assert.NoError(t, err)
	assert.False(t, batch.PubUpdates.ContainsPostOrderWrites)
}

func TestTxStatsInfo(t *testing.T) {
	testDBEnv := &privacyenabledstate.LevelDBCommonStorageTestEnv{}
	testDBEnv.Init(t)
	defer testDBEnv.Cleanup()
	testDB := testDBEnv.GetDBHandle("emptydb")

	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	v := NewStatebasedValidator(nil, testDB, nil, cryptoProvider)

	// create a block with 4 endorser transactions
	tx1SimulationResults, _ := testutilGenerateTxSimulationResultsAsBytes(t,
		&testRwset{
			writes: []*testKeyWrite{
				{ns: "ns1", key: "key1", val: "val1"},
			},
		},
	)
	tx2SimulationResults, _ := testutilGenerateTxSimulationResultsAsBytes(t,
		&testRwset{
			reads: []*testKeyRead{
				{ns: "ns1", key: "key1", version: nil}, // should cause mvcc read-conflict with tx1
			},
		},
	)
	tx3SimulationResults, _ := testutilGenerateTxSimulationResultsAsBytes(t,
		&testRwset{
			writes: []*testKeyWrite{
				{ns: "ns1", key: "key2", val: "val2"},
			},
		},
	)
	tx4SimulationResults, _ := testutilGenerateTxSimulationResultsAsBytes(t,
		&testRwset{
			writes: []*testKeyWrite{
				{ns: "ns1", coll: "coll1", key: "key1", val: "val1"},
				{ns: "ns1", coll: "coll2", key: "key1", val: "val1"},
			},
		},
	)

	blockDetails := &testutil.BlockDetails{
		BlockNum:     5,
		PreviousHash: []byte("previousHash"),
		Txs: []*testutil.TxDetails{
			{
				TxID:              "tx_1",
				ChaincodeName:     "cc_1",
				ChaincodeVersion:  "cc_1_v1",
				SimulationResults: tx1SimulationResults,
				Type:              common.HeaderType_ENDORSER_TRANSACTION,
			},
			{
				TxID:              "tx_2",
				ChaincodeName:     "cc_2",
				ChaincodeVersion:  "cc_2_v1",
				SimulationResults: tx2SimulationResults,
				Type:              common.HeaderType_ENDORSER_TRANSACTION,
			},
			{
				TxID:              "tx_3",
				ChaincodeName:     "cc_3",
				ChaincodeVersion:  "cc_3_v1",
				SimulationResults: tx3SimulationResults,
				Type:              common.HeaderType_ENDORSER_TRANSACTION,
			},
			{
				TxID:              "tx_4",
				ChaincodeName:     "cc_4",
				ChaincodeVersion:  "cc_4_v1",
				SimulationResults: tx4SimulationResults,
				Type:              common.HeaderType_ENDORSER_TRANSACTION,
			},
		},
	}

	block := testutil.ConstructBlockFromBlockDetails(t, blockDetails, false)
	txsFilter := lutils.NewTxValidationFlags(4)
	txsFilter.SetFlag(0, peer.TxValidationCode_VALID)
	txsFilter.SetFlag(1, peer.TxValidationCode_VALID)
	txsFilter.SetFlag(2, peer.TxValidationCode_BAD_PAYLOAD)
	txsFilter.SetFlag(3, peer.TxValidationCode_VALID)
	block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER] = txsFilter

	// collect the validation stats for the block and check against the expected stats
	_, txStatsInfo, err := v.ValidateAndPrepareBatch(&ledger.BlockAndPvtData{Block: block}, true)
	assert.NoError(t, err)
	expectedTxStatInfo := []*txmgr.TxStatInfo{
		{
			TxType:         common.HeaderType_ENDORSER_TRANSACTION,
			ValidationCode: peer.TxValidationCode_VALID,
			ChaincodeID:    &peer.ChaincodeID{Name: "cc_1", Version: "cc_1_v1"},
		},
		{
			TxType:         common.HeaderType_ENDORSER_TRANSACTION,
			ValidationCode: peer.TxValidationCode_MVCC_READ_CONFLICT,
			ChaincodeID:    &peer.ChaincodeID{Name: "cc_2", Version: "cc_2_v1"},
		},
		{
			TxType:         -1,
			ValidationCode: peer.TxValidationCode_BAD_PAYLOAD,
		},
		{
			TxType:         common.HeaderType_ENDORSER_TRANSACTION,
			ValidationCode: peer.TxValidationCode_VALID,
			ChaincodeID:    &peer.ChaincodeID{Name: "cc_4", Version: "cc_4_v1"},
			NumCollections: 2,
		},
	}
	t.Logf("txStatsInfo=%s\n", spew.Sdump(txStatsInfo))
	assert.Equal(t, expectedTxStatInfo, txStatsInfo)
}

func testutilSampleTxSimulationResults(t *testing.T, key string) *ledger.TxSimulationResults {
	rwSetBuilder := rwsetutil.NewRWSetBuilder()
	// public rws ns1 + ns2
	rwSetBuilder.AddToReadSet("ns1", key, version.NewHeight(1, 1))
	rwSetBuilder.AddToReadSet("ns2", key, version.NewHeight(1, 1))
	rwSetBuilder.AddToWriteSet("ns2", key, []byte("ns2-key1-value"))

	// pvt rwset ns1
	rwSetBuilder.AddToHashedReadSet("ns1", "coll1", key, version.NewHeight(1, 1))
	rwSetBuilder.AddToHashedReadSet("ns1", "coll2", key, version.NewHeight(1, 1))
	rwSetBuilder.AddToPvtAndHashedWriteSet("ns1", "coll2", key, []byte("pvt-ns1-coll2-key1-value"))

	// pvt rwset ns2
	rwSetBuilder.AddToHashedReadSet("ns2", "coll1", key, version.NewHeight(1, 1))
	rwSetBuilder.AddToHashedReadSet("ns2", "coll2", key, version.NewHeight(1, 1))
	rwSetBuilder.AddToPvtAndHashedWriteSet("ns2", "coll2", key, []byte("pvt-ns2-coll2-key1-value"))
	rwSetBuilder.AddToPvtAndHashedWriteSet("ns2", "coll3", key, nil)

	rwSetBuilder.AddToHashedReadSet("ns3", "coll1", key, version.NewHeight(1, 1))

	pubAndPvtSimulationResults, err := rwSetBuilder.GetTxSimulationResults()
	if err != nil {
		t.Fatalf("ConstructSimulationResultsWithPvtData failed while getting simulation results, err %s", err)
	}

	return pubAndPvtSimulationResults
}

type testKeyRead struct {
	ns, coll, key string
	version       *version.Height
}
type testKeyWrite struct {
	ns, coll, key string
	val           string
}
type testRwset struct {
	reads  []*testKeyRead
	writes []*testKeyWrite
}

func testutilGenerateTxSimulationResults(t *testing.T, rwsetInfo *testRwset) *ledger.TxSimulationResults {
	rwSetBuilder := rwsetutil.NewRWSetBuilder()
	for _, r := range rwsetInfo.reads {
		if r.coll == "" {
			rwSetBuilder.AddToReadSet(r.ns, r.key, r.version)
		} else {
			rwSetBuilder.AddToHashedReadSet(r.ns, r.coll, r.key, r.version)
		}
	}

	for _, w := range rwsetInfo.writes {
		if w.coll == "" {
			rwSetBuilder.AddToWriteSet(w.ns, w.key, []byte(w.val))
		} else {
			rwSetBuilder.AddToPvtAndHashedWriteSet(w.ns, w.coll, w.key, []byte(w.val))
		}
	}
	simulationResults, err := rwSetBuilder.GetTxSimulationResults()
	assert.NoError(t, err)
	return simulationResults
}

func testutilGenerateTxSimulationResultsAsBytes(
	t *testing.T, rwsetInfo *testRwset) (
	publicSimulationRes []byte, pvtWS []byte,
) {
	simulationRes := testutilGenerateTxSimulationResults(t, rwsetInfo)
	pub, err := simulationRes.GetPubSimulationBytes()
	assert.NoError(t, err)
	pvt, err := simulationRes.GetPvtSimulationBytes()
	assert.NoError(t, err)
	return pub, pvt
}

//go:generate counterfeiter -o mock/txsim.go --fake-name TxSimulator . txSimulator
type txSimulator interface {
	ledger.TxSimulator
}

//go:generate counterfeiter -o mock/processor.go --fake-name Processor . processor
type processor interface {
	ledger.CustomTxProcessor
}

//go:generate counterfeiter -o mock/txmgr.go --fake-name TxMgr . txMgr
type txMgr interface {
	txmgr.TxMgr
}

// Test for txType != common.HeaderType_ENDORSER_TRANSACTION
func Test_preprocessProtoBlock_processNonEndorserTx(t *testing.T) {
	// Register customtx processor
	mockTxProcessor := new(mock.Processor)
	mockTxProcessor.GenerateSimulationResultsReturns(nil)
	customTxProcessors := map[common.HeaderType]ledger.CustomTxProcessor{
		100: mockTxProcessor,
	}

	// Prepare param1: txmgr.TxMgr
	kvw := &kvrwset.KVWrite{Key: "key1", IsDelete: false, Value: []byte{0xde, 0xad, 0xbe, 0xef}}
	kvrw := &kvrwset.KVRWSet{Writes: []*kvrwset.KVWrite{kvw}}
	mkvrw, _ := proto.Marshal(kvrw)
	nrws := rwset.NsReadWriteSet{
		Namespace: "ns1",
		Rwset:     mkvrw,
	}
	pubsimresults := rwset.TxReadWriteSet{
		DataModel: -1,
		NsRwset:   []*rwset.NsReadWriteSet{&nrws},
	}
	txsimres := &ledger.TxSimulationResults{
		PubSimulationResults: &pubsimresults,
		PvtSimulationResults: nil,
	}
	txsim_ := new(mock.TxSimulator)
	txsim_.GetTxSimulationResultsReturns(txsimres, nil)
	txmgr_ := new(mock.TxMgr)
	txmgr_.NewTxSimulatorReturns(txsim_, nil)

	// Prepare param2: validateKVFunc
	alwaysValidKVFunc := func(key string, value []byte) error {
		return nil
	}

	// Prepare param3: *common.Block
	pubSimulationResults := [][]byte{}
	txids := []string{"tx1"}
	// Get simulation results for tx1
	rwSetBuilder := rwsetutil.NewRWSetBuilder()
	tx1SimulationResults, err := rwSetBuilder.GetTxSimulationResults()
	assert.NoError(t, err)
	// Add tx1 public rwset to the set of results
	res, err := tx1SimulationResults.GetPubSimulationBytes()
	assert.NoError(t, err)
	pubSimulationResults = append(pubSimulationResults, res)
	// Construct a block using a transaction simulation result
	block := testutil.ConstructBlockWithTxidHeaderType(
		t,
		10,
		testutil.ConstructRandomBytes(t, 32),
		pubSimulationResults,
		txids,
		false,
		100,
	)

	// Call
	internalBlock, txsStatInfo, err2 := preprocessProtoBlock(txmgr_, alwaysValidKVFunc, block, false, customTxProcessors)

	// Prepare expected value
	expectedPreprocessedBlock := &internal.Block{
		Num: 10,
	}
	value1 := []byte{0xde, 0xad, 0xbe, 0xef}
	expKVWrite := &kvrwset.KVWrite{
		Key:      "key1",
		IsDelete: false,
		Value:    value1,
	}
	expKVRWSet := &kvrwset.KVRWSet{
		Writes: []*kvrwset.KVWrite{expKVWrite},
	}
	expNsRwSet := &rwsetutil.NsRwSet{
		NameSpace: "ns1",
		KvRwSet:   expKVRWSet,
	}
	expTxRwSet := &rwsetutil.TxRwSet{
		NsRwSets: []*rwsetutil.NsRwSet{expNsRwSet},
	}
	expectedPreprocessedBlock.Txs = append(
		expectedPreprocessedBlock.Txs,
		&internal.Transaction{
			IndexInBlock:            0,
			ID:                      "tx1",
			RWSet:                   expTxRwSet,
			ContainsPostOrderWrites: true,
		},
	)
	expectedTxStatInfo := []*txmgr.TxStatInfo{
		{
			TxType: 100,
		},
	}

	// Check result
	assert.NoError(t, err2)
	assert.Equal(t, expectedPreprocessedBlock, internalBlock)
	assert.Equal(t, expectedTxStatInfo, txsStatInfo)
}
