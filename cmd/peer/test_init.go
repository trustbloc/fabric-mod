// +build testing

/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	xtestutil "github.com/hyperledger/fabric/extensions/testutil"
)

// init is only executed for the unit test
func init() {
	xtestutil.SetupResources()
}
