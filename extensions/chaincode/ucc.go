/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode

import (
	"github.com/hyperledger/fabric/extensions/chaincode/api"
)

// GetUCC returns the in-process user chaincode for the given name and version
func GetUCC(name, version string) (api.UserCC, bool) {
	return nil, false
}

// GetUCCByID returns the in-process user chaincode for the given ID
func GetUCCByID(string) (api.UserCC, bool) {
	return nil, false
}

// Chaincodes returns all registered in-process chaincodes
func Chaincodes() []api.UserCC {
	return nil
}

// WaitForReady blocks until the chaincodes are all registered
func WaitForReady() {
}

// GetID returns the ID of the chaincode which includes the name and version
func GetID(cc api.UserCC) string {
	return cc.Name() + ":" + cc.Version()
}
