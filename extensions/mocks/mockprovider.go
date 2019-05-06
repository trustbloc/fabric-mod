/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package mocks

import (
	storeapi "github.com/hyperledger/fabric/extensions/collections/api/store"
)

// DataProvider is a mock data provider
type DataProvider struct {
}

func (p *DataProvider) RetrieverForChannel(channel string) storeapi.Retriever {
	return &dataRetriever{}
}

type dataRetriever struct {
}
