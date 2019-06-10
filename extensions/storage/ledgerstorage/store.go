/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package ledgerstorage

//SyncPvtdataStoreWithBlockStoreHandler provides extension for syncing private data store with block store
func SyncPvtdataStoreWithBlockStoreHandler(handle func() error) func() error {
	return handle
}
