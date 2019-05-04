/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package transientstore

import "github.com/hyperledger/fabric/core/transientstore"

func NewStoreProvider() transientstore.StoreProvider {
	return transientstore.NewStoreProvider()
}
