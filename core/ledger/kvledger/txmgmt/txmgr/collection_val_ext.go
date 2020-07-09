/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package txmgr

import (
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/core/ledger"
)

func (v *collNameValidator) getCollConfig(ns, coll string) (*peer.CollectionConfig, error) {
	if !v.cache.isPopulatedFor(ns) {
		conf, err := v.retrieveCollConfigFromStateDB(ns)
		if err != nil {
			return nil, err
		}
		v.cache.populate(ns, conf)
	}
	config, ok := v.cache.getCollConfig(ns, coll)
	if !ok {
		return nil, &ledger.InvalidCollNameError{
			Ns:   ns,
			Coll: coll,
		}
	}
	return config, nil
}

func (c collConfigCache) getCollConfig(ns, coll string) (*peer.CollectionConfig, bool) {
	config, ok := c[collConfigkey{ns, coll}]
	return config, ok
}
