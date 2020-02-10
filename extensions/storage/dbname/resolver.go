/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package dbname

// Resolve resolves the database name. This default implementation simply returns the provided DB name.
func Resolve(dbName string) string {
	return dbName
}
