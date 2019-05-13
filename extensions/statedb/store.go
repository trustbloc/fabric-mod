/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package privacyenabledstate

import (
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/statedb"
)

// NewVersionedDBProvider instantiates VersionedDBProvider
func NewVersionedDBProvider(vdbProvider statedb.VersionedDBProvider) statedb.VersionedDBProvider {
	return vdbProvider
}
