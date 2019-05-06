/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package store

import (
	"fmt"
)

// Key is a key for retrieving collection data
type Key struct {
	EndorsedAtTxID string
	Namespace      string
	Collection     string
	Key            string
}

// NewKey returns a new collection key
func NewKey(endorsedAtTxID string, ns string, coll string, key string) *Key {
	return &Key{
		EndorsedAtTxID: endorsedAtTxID,
		Namespace:      ns,
		Collection:     coll,
		Key:            key,
	}
}

// String returns the string representation of the key
func (k *Key) String() string {
	return fmt.Sprintf("%s:%s:%s-%s", k.Namespace, k.Collection, k.Key, k.EndorsedAtTxID)
}

// MultiKey is a key for retrieving collection data for multiple keys
type MultiKey struct {
	EndorsedAtTxID string
	Namespace      string
	Collection     string
	Keys           []string
}

// NewMultiKey returns a new collection data multi-key
func NewMultiKey(endorsedAtTxID string, ns string, coll string, keys ...string) *MultiKey {
	return &MultiKey{
		EndorsedAtTxID: endorsedAtTxID,
		Namespace:      ns,
		Collection:     coll,
		Keys:           keys,
	}
}

// String returns the string representation of the key
func (k *MultiKey) String() string {
	return fmt.Sprintf("%s:%s:%s-%s", k.Namespace, k.Collection, k.Keys, k.EndorsedAtTxID)
}
