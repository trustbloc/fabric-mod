/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode

import (
	"github.com/hyperledger/fabric/extensions/chaincode/api"
	"github.com/hyperledger/fabric/msp"
)

// GetUCC returns the in-process user chaincode for the given name and version
func GetUCC(name, version string) (api.UserCC, bool) {
	return nil, false
}

// GetUCCByPackageID returns the in-process user chaincode for the given package ID
func GetUCCByPackageID(string) (api.UserCC, bool) {
	return nil, false
}

// Chaincodes returns all registered in-process chaincodes
func Chaincodes() []api.UserCC {
	return nil
}

// WaitForReady blocks until the chaincodes are all registered
func WaitForReady() {
}

// GetPackageID returns the package ID of the chaincode
func GetPackageID(cc api.UserCC) string {
	return cc.Name() + ":" + cc.Version()
}

// IsValidMSP return true if the given MSP is valid for chaincode/collection policy
func IsValidMSP(mspID string, msps map[string]msp.MSP) bool {
	_, ok := msps[mspID]
	return ok
}
