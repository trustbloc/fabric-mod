/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package endorser

import (
	"testing"

	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/ledger/rwset"
	"github.com/stretchr/testify/assert"
)

func TestFilterPubSimulationResults(t *testing.T) {
	pubSimulationResults := &rwset.TxReadWriteSet{}
	p, err := FilterPubSimulationResults(map[string]*common.CollectionConfigPackage{}, pubSimulationResults)
	assert.NoError(t, err)
	assert.Equal(t, pubSimulationResults, p)
}
