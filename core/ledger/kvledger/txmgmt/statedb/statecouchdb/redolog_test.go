/*
Copyright IBM Corp. All Rights Reserved.
SPDX-License-Identifier: Apache-2.0
*/

package statecouchdb

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/statedb"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/version"
	"github.com/stretchr/testify/assert"
)

func TestRedoLogger(t *testing.T) {
	provider, cleanup := redologTestSetup(t)
	defer cleanup()

	loggers := []*redoLogger{}
	records := []*redoRecord{}

	verifyLogRecords := func() {
		for i := 0; i < len(loggers); i++ {
			retrievedRec, err := loggers[i].load()
			assert.NoError(t, err)
			assert.Equal(t, records[i], retrievedRec)
		}
	}

	// write log records for multiple channels
	for i := 0; i < 10; i++ {
		logger := provider.newRedoLogger(fmt.Sprintf("channel-%d", i))
		rec, err := logger.load()
		assert.NoError(t, err)
		assert.Nil(t, rec)
		loggers = append(loggers, logger)
		batch := statedb.NewUpdateBatch()
		blkNum := uint64(i)
		batch.Put("ns1", "key1", []byte("value1"), version.NewHeight(blkNum, 1))
		batch.Put("ns2", string([]byte{0x00, 0xff}), []byte("value3"), version.NewHeight(blkNum, 3))
		batch.PutValAndMetadata("ns2", string([]byte{0x00, 0xff}), []byte("value3"), []byte("metadata"), version.NewHeight(blkNum, 4))
		batch.Delete("ns2", string([]byte{0xff, 0xff}), version.NewHeight(blkNum, 5))
		rec = &redoRecord{
			UpdateBatch: batch,
			Version:     version.NewHeight(blkNum, 10),
		}
		records = append(records, rec)
		assert.NoError(t, logger.persist(rec))
	}

	verifyLogRecords()
	// overwrite logrecord for one channel
	records[5].UpdateBatch = statedb.NewUpdateBatch()
	records[5].Version = version.NewHeight(5, 5)
	assert.NoError(t, loggers[5].persist(records[5]))
	verifyLogRecords()
}

func TestCouchdbRedoLogger(t *testing.T) {
	testEnv := NewTestVDBEnv(t)
	defer testEnv.Cleanup()

	// commitToRedologAndRestart - a helper function that commits directly to redologs and restart the statedb
	commitToRedologAndRestart := func(newVal string, version *version.Height) {
		batch := statedb.NewUpdateBatch()
		batch.Put("ns1", "key1", []byte(newVal), version)
		db, err := testEnv.DBProvider.GetDBHandle("testcouchdbredologger")
		assert.NoError(t, err)
		vdb := db.(*VersionedDB)
		assert.NoError(t,
			vdb.redoLogger.persist(
				&redoRecord{
					UpdateBatch: batch,
					Version:     version,
				},
			),
		)
		testEnv.CloseAndReopen()
	}
	// verifyExpectedVal - a helper function that verifies the statedb contents
	verifyExpectedVal := func(expectedVal string, expectedSavepoint *version.Height) {
		db, err := testEnv.DBProvider.GetDBHandle("testcouchdbredologger")
		assert.NoError(t, err)
		vdb := db.(*VersionedDB)
		vv, err := vdb.GetState("ns1", "key1")
		assert.NoError(t, err)
		assert.Equal(t, expectedVal, string(vv.Value))
		savepoint, err := vdb.GetLatestSavePoint()
		assert.NoError(t, err)
		assert.Equal(t, expectedSavepoint, savepoint)
	}

	// initialize statedb with initial set of writes
	db, err := testEnv.DBProvider.GetDBHandle("testcouchdbredologger")
	if err != nil {
		t.Fatalf("Failed to get database handle: %s", err)
	}
	vdb := db.(*VersionedDB)
	batch1 := statedb.NewUpdateBatch()
	batch1.Put("ns1", "key1", []byte("value1"), version.NewHeight(1, 1))
	vdb.ApplyUpdates(batch1, version.NewHeight(1, 1))

	// make redolog one block ahead than statedb - upon restart the redolog should get applied
	commitToRedologAndRestart("value2", version.NewHeight(2, 1))
	verifyExpectedVal("value2", version.NewHeight(2, 1))

	// make redolog two blocks ahead than statedb - upon restart the redolog should be ignored
	commitToRedologAndRestart("value3", version.NewHeight(4, 1))
	verifyExpectedVal("value2", version.NewHeight(2, 1))

	// make redolog one block behind than statedb - upon restart the redolog should be ignored
	commitToRedologAndRestart("value3", version.NewHeight(1, 5))
	verifyExpectedVal("value2", version.NewHeight(2, 1))

	// A nil height should cause skipping the writing of redo-record
	db, _ = testEnv.DBProvider.GetDBHandle("testcouchdbredologger")
	vdb = db.(*VersionedDB)
	vdb.ApplyUpdates(batch1, nil)
	record, err := vdb.redoLogger.load()
	assert.NoError(t, err)
	assert.Equal(t, version.NewHeight(1, 5), record.Version)
	assert.Equal(t, []byte("value3"), record.UpdateBatch.Get("ns1", "key1").Value)

	// A batch that does not contain PostOrderWrites should cause skipping the writing of redo-record
	db, _ = testEnv.DBProvider.GetDBHandle("testcouchdbredologger")
	vdb = db.(*VersionedDB)
	batchWithNoGeneratedWrites := batch1
	batchWithNoGeneratedWrites.ContainsPostOrderWrites = false
	vdb.ApplyUpdates(batchWithNoGeneratedWrites, version.NewHeight(2, 5))
	record, err = vdb.redoLogger.load()
	assert.NoError(t, err)
	assert.Equal(t, version.NewHeight(1, 5), record.Version)
	assert.Equal(t, []byte("value3"), record.UpdateBatch.Get("ns1", "key1").Value)

	// A batch that contains PostOrderWrites should cause writing of redo-record
	db, _ = testEnv.DBProvider.GetDBHandle("testcouchdbredologger")
	vdb = db.(*VersionedDB)
	batchWithGeneratedWrites := batch1
	batchWithGeneratedWrites.ContainsPostOrderWrites = true
	vdb.ApplyUpdates(batchWithNoGeneratedWrites, version.NewHeight(3, 4))
	record, err = vdb.redoLogger.load()
	assert.NoError(t, err)
	assert.Equal(t, version.NewHeight(3, 4), record.Version)
	assert.Equal(t, []byte("value1"), record.UpdateBatch.Get("ns1", "key1").Value)
}

func redologTestSetup(t *testing.T) (p *redoLoggerProvider, cleanup func()) {
	dbPath, err := ioutil.TempDir("", "redolog")
	if err != nil {
		t.Fatalf("Failed to create redo log directory: %s", err)
	}
	p, err = newRedoLoggerProvider(dbPath)
	assert.NoError(t, err)
	cleanup = func() {
		p.close()
		assert.NoError(t, os.RemoveAll(dbPath))
	}
	return
}

// testGenerareRedoRecord is the code that generates a serialized redo record into a
// file based on the current version of the code, so that the file with serialized data
// can get checked into source control. The following test function
// 'TestReadExistingRedoRecord' verifies data compatibility in later builds/releases.
// Specifically, it verifies that the changes in the struct statedb.NewUpdateBatch
// are compatible such that the redo records persisted from the earlier commit/release
// can still be deserialized on later commits/releases.
// In order to generate this serialized record, change this function name to start with
// uppercase "T" so that execution of go test will generate the test file.
func testGenerareRedoRecord(t *testing.T) {
	val, err := encodeRedologVal(constructSampleRedoRecord())
	assert.NoError(t, err)
	assert.NoError(t, ioutil.WriteFile("testdata/persisted_redo_record", val, 0644))
}

func TestReadExistingRedoRecord(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/persisted_redo_record")
	assert.NoError(t, err)
	rec, err := decodeRedologVal(b)
	assert.NoError(t, err)
	t.Logf("rec = %s", spew.Sdump(rec))
	assert.Equal(t, constructSampleRedoRecord(), rec)
}

func constructSampleRedoRecord() *redoRecord {
	batch := statedb.NewUpdateBatch()
	batch.Put("ns1", "key1", []byte("value1"), version.NewHeight(1, 1))
	batch.Put("ns2", string([]byte{0x00, 0xff}), []byte("value3"), version.NewHeight(3, 3))
	batch.PutValAndMetadata("ns2", string([]byte{0x00, 0xff}), []byte("value3"), []byte("metadata"), version.NewHeight(4, 4))
	batch.Delete("ns2", string([]byte{0xff, 0xff}), version.NewHeight(5, 5))
	return &redoRecord{
		UpdateBatch: batch,
		Version:     version.NewHeight(10, 10),
	}
}
