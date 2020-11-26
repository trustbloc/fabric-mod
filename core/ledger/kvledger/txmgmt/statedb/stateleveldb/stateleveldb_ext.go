/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package stateleveldb

// UpdateCache is not implemented
func (vdb *versionedDB) UpdateCache(uint64, []byte) error {
	return nil
}
