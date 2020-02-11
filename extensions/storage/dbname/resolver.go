/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package dbname

// Resolve resolves the database name. This default implementation simply returns the provided DB name.
func Resolve(dbName string) string {
	return dbName
}

// IsRelevant returns true if the given database is relevant to this peer. If the database is shared
// by multiple peers then it may not be relevant to this peer.
// This default implementation simply returns true.
func IsRelevant(dbName string) bool {
	return true
}
