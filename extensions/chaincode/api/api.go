/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"github.com/hyperledger/fabric/core/scc"
)

// DBArtifacts defines DB artifacts, including indexes
type DBArtifacts struct {
	Indexes           []string
	CollectionIndexes map[string][]string
}

// UserCC contains information about an in-process user chaincode
type UserCC interface {
	scc.SelfDescribingSysCC

	// Version returns the version of this chaincode
	Version() string
	// GetDBArtifacts returns the DB artifacts for this chaincode along with those of the given collections
	GetDBArtifacts(collNames []string) map[string]*DBArtifacts
}
