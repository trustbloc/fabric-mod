/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package endorser

import (
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/ledger/rwset"
)

// FilterPubSimulationResults filters the public simulation results and returns the filtered results or error.
// The default implementation does not perform any filtering.
func FilterPubSimulationResults(_ map[string]*common.CollectionConfigPackage, pubSimulationResults *rwset.TxReadWriteSet) (*rwset.TxReadWriteSet, error) {
	return pubSimulationResults, nil
}
