/*
Copyright IBM Corp. 2017 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bookkeeping

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestEnv provides the bookkeeper provider env for testing
type TestEnv struct {
	t            testing.TB
	TestProvider Provider
	dbPath       string
}

// NewTestEnv construct a TestEnv for testing
func NewTestEnv(t testing.TB) *TestEnv {
	dbPath, err := ioutil.TempDir("", "bookkeep")
	require.NoError(t, err)
	provider, err := NewProvider(dbPath)
	require.NoError(t, err)
	return &TestEnv{t, provider, dbPath}
}

// Cleanup cleansup the  store env after testing
func (te *TestEnv) Cleanup() {
	te.TestProvider.Close()
	os.RemoveAll(te.dbPath)
}
