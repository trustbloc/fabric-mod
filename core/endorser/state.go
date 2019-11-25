/*
Copyright IBM Corp. 2018 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package endorser

import (
	"github.com/hyperledger/fabric-protos-go/ledger/rwset"
	endorsement "github.com/hyperledger/fabric/core/handlers/endorsement/api/state"
	"github.com/hyperledger/fabric/core/ledger"
	storageapi "github.com/hyperledger/fabric/extensions/storage/api"
	"github.com/pkg/errors"
)

//go:generate mockery -dir . -name QueryCreator -case underscore -output mocks/
// QueryCreator creates new QueryExecutors
type QueryCreator interface {
	NewQueryExecutor() (ledger.QueryExecutor, error)
}

// ChannelState defines state operations
type ChannelState struct {
	storageapi.TransientStore
	QueryCreator
}

// FetchState fetches state
func (cs *ChannelState) FetchState() (endorsement.State, error) {
	qe, err := cs.NewQueryExecutor()
	if err != nil {
		return nil, err
	}

	return &StateContext{
		QueryExecutor:  qe,
		TransientStore: cs.TransientStore,
	}, nil
}

// StateContext defines an execution context that interacts with the state
type StateContext struct {
	storageapi.TransientStore
	ledger.QueryExecutor
}

// GetTransientByTXID returns the private data associated with this transaction ID.
func (sc *StateContext) GetTransientByTXID(txID string) ([]*rwset.TxPvtReadWriteSet, error) {
	scanner, err := sc.TransientStore.GetTxPvtRWSetByTxid(txID, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer scanner.Close()
	var data []*rwset.TxPvtReadWriteSet
	for {
		res, err := scanner.Next()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if res == nil {
			break
		}
		if res.PvtSimulationResultsWithConfig == nil {
			continue
		}
		data = append(data, res.PvtSimulationResultsWithConfig.PvtRwset)
	}
	return data, nil
}
