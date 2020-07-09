/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package txmgr

import (
	"github.com/pkg/errors"

	commonledger "github.com/hyperledger/fabric/common/ledger"
)

// GetPrivateData implements method in interface `ledger.QueryExecutor`
func (q *queryExecutor) GetPrivateData(ns, coll, key string) ([]byte, error) {
	if err := q.checkDone(); err != nil {
		return nil, err
	}

	config, err := q.collNameValidator.getCollConfig(ns, coll)
	if err != nil {
		return nil, err
	}

	staticConfig := config.GetStaticCollectionConfig()
	if staticConfig == nil {
		return nil, errors.New("invalid collection config")
	}

	value, handled, err := q.pvtDataHandler.HandleGetPrivateData(q.txid, ns, staticConfig, key)
	if err != nil {
		return nil, err
	}

	if handled {
		return value, nil
	}

	return q.getPrivateData(ns, coll, key)
}

// GetPrivateDataMultipleKeys implements method in interface `ledger.QueryExecutor`
func (q *queryExecutor) GetPrivateDataMultipleKeys(ns, coll string, keys []string) ([][]byte, error) {
	if err := q.checkDone(); err != nil {
		return nil, err
	}

	config, err := q.collNameValidator.getCollConfig(ns, coll)
	if err != nil {
		return nil, err
	}

	staticConfig := config.GetStaticCollectionConfig()
	if staticConfig == nil {
		return nil, errors.New("invalid collection config")
	}

	value, handled, err := q.pvtDataHandler.HandleGetPrivateDataMultipleKeys(q.txid, ns, staticConfig, keys)
	if err != nil {
		return nil, err
	}

	if handled {
		return value, nil
	}

	return q.getPrivateDataMultipleKeys(ns, coll, keys)
}

// ExecuteQueryOnPrivateData implements method in interface `ledger.QueryExecutor`
func (q *queryExecutor) ExecuteQueryOnPrivateData(ns, coll, query string) (commonledger.ResultsIterator, error) {
	if err := q.checkDone(); err != nil {
		return nil, err
	}

	config, err := q.collNameValidator.getCollConfig(ns, coll)
	if err != nil {
		return nil, err
	}

	staticConfig := config.GetStaticCollectionConfig()
	if staticConfig == nil {
		return nil, errors.New("invalid collection config")
	}

	it, handled, err := q.pvtDataHandler.HandleExecuteQueryOnPrivateData(q.txid, ns, staticConfig, query)
	if err != nil {
		return nil, err
	}

	if handled {
		return it, nil
	}

	return q.executeQueryOnPrivateData(ns, coll, query)
}
