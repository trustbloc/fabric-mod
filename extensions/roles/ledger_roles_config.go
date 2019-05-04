/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package roles

// IsCommitter returns true if the peer is a committer, otherwise the peer does not commit to the DB
func IsCommitter() bool {
	return true
}

// IsEndorser returns true if the peer is an endorser
func IsEndorser() bool {
	return true
}

// IsValidator returns true if the peer is a validator
func IsValidator() bool {
	return true
}

// RolesAsString returns the roles for the peer
func RolesAsString() []string {
	return []string{}
}
