/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package config

// IsSkipCheckForDupTxnID indicates whether or not endorsers should skip the check for duplicate transactions IDs. The check
// would still be performed during validation.
func IsSkipCheckForDupTxnID() bool {
	return false
}
