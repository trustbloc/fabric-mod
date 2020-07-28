/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package testutil

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/statedb"
)

//SetupExtTestEnv creates new extension test environment,
// it creates couchdb instance for test, returns couchdbd address, cleanup and destroy function handle.
func SetupExtTestEnv() (addr string, cleanup func(string), stop func()) {
	return "", func(string) {
			//do nothing
		}, func() {
			//do nothing
		}
}

// SetupResources sets up all of the mock resource providers
func SetupResources() func() {
	return func() {
		//do nothing
	}
}

func GetExtStateDBProvider(t testing.TB, dbProvider statedb.VersionedDBProvider) statedb.VersionedDBProvider {
	return nil
}

// SetupV2Data setup v2 data
func SetupV2Data(t *testing.T, config *ledger.Config) func() {
	require.NoError(t, unzip("testdata/v20/sample_ledgers/ledgersData.zip", config.RootFSPath, false))

	return func() {
		//do nothing
	}
}

func unzip(src string, dest string, createTopLevelDirInZip bool) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	// iterate all the dirs and files in the zip file
	for _, file := range r.File {
		filePath := file.Name
		if !createTopLevelDirInZip {
			// trim off the top level dir - for example, trim ledgersData/historydb/abc to historydb/abc
			index := strings.Index(filePath, string(filepath.Separator))
			filePath = filePath[index+1:]
		}

		fullPath := filepath.Join(dest, filePath)
		if file.FileInfo().IsDir() {
			os.MkdirAll(fullPath, os.ModePerm)
			continue
		}
		if err = os.MkdirAll(filepath.Dir(fullPath), os.ModePerm); err != nil {
			return err
		}
		outFile, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

// MarbleValue return marble value
func MarbleValue(name, color, owner string, size int) string {
	return fmt.Sprintf(`{"docType":"marble","name":"%s","color":"%s","size":%d,"owner":"%s"}`, name, color, size, owner)
}

func TestLedgerConf() *ledger.Config {
	conf := &ledger.Config{
		RootFSPath: "",
		StateDBConfig: &ledger.StateDBConfig{
			CouchDB: &ledger.CouchDBConfig{},
		},
		PrivateDataConfig: &ledger.PrivateDataConfig{},
		HistoryDBConfig:   &ledger.HistoryDBConfig{},
	}

	return conf
}

// Skip skips the unit test for extensions
func Skip(t *testing.T, msg string) {
}
