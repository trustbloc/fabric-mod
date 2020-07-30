/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package policy

import (
	"github.com/hyperledger/fabric-protos-go/peer"
)

// Validator is a noop collection policy validator
type Validator struct {
}

// NewValidator returns a noop collection policy validator
func NewValidator() *Validator {
	return &Validator{}
}

// Validator validates various collection config types
func (v *Validator) Validate(collConfig *peer.CollectionConfig) error {
	return nil
}

// ValidateNewCollectionConfigsAgainstOld validates updated collection configs
func (v *Validator) ValidateNewCollectionConfigsAgainstOld(newCollectionConfigs []*peer.CollectionConfig, oldCollectionConfigs []*peer.CollectionConfig) error {
	return nil
}

// ValidateCollectionConfig validates a new collection config
func (v *Validator) ValidateCollectionConfig(*peer.StaticCollectionConfig) error {
	return nil
}

// ValidateNewCollectionConfigAgainstCommitted validates a new collection config against a committed collection config
func (v *Validator) ValidateNewCollectionConfigAgainstCommitted(newColl, committedColl *peer.StaticCollectionConfig) error {
	return nil
}
