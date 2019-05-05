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

package ledgerstorage

import (
	"os"
	"testing"

	"github.com/hyperledger/fabric/core/ledger/ledgerconfig"
	ledgertestutil "github.com/hyperledger/fabric/core/ledger/testutil"
	xtestutil "github.com/hyperledger/fabric/extensions/testutil"
)

type testEnv struct {
	t                 testing.TB
	cleanupExtTestEnv func()
}

func newTestEnv(t *testing.T) *testEnv {
	// Read the core.yaml file for default config.
	ledgertestutil.SetupCoreYAMLConfig()

	//setup extension test environment
	_, _, destroy := xtestutil.SetupExtTestEnv()

	testEnv := &testEnv{t, destroy}
	path := ledgerconfig.GetRootPath()
	os.RemoveAll(path)
	return testEnv
}

func (env *testEnv) cleanup() {
	path := ledgerconfig.GetRootPath()
	os.RemoveAll(path)
	env.cleanupExtTestEnv()
}
