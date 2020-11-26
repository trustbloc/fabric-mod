/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package config

// IsPrePopulateStateCache indicates whether or not the state cache on the endorsing peer should be pre-populated
// with values retrieved from the committing peer.
func IsPrePopulateStateCache() bool {
	return false
}

// IsSaveCacheUpdates indicates whether or not state updates should be saved on the committing peer.
func IsSaveCacheUpdates() bool {
	return false
}

// IsSkipCheckForDupTxnID indicates whether or not endorsers should skip the check for duplicate transactions IDs. The check
// would still be performed during validation.
func IsSkipCheckForDupTxnID() bool {
	return false
}
