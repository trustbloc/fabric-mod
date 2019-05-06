/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package lockbasedtxmgr

import (
	"github.com/pkg/errors"
)

func (h *queryHelper) handleGetPrivateData(txID, ns, coll, key string) ([]byte, error) {
	if err := h.checkDone(); err != nil {
		return nil, err
	}

	config, err := h.collNameValidator.getCollConfig(ns, coll)
	if err != nil {
		return nil, err
	}

	staticConfig := config.GetStaticCollectionConfig()
	if staticConfig == nil {
		return nil, errors.New("invalid collection config")
	}

	value, handled, err := h.pvtDataHandler.HandleGetPrivateData(txID, ns, staticConfig, key)
	if err != nil {
		return nil, err
	}

	if handled {
		return value, nil
	}

	return h.getPrivateData(ns, coll, key)
}

func (h *queryHelper) handleGetPrivateDataMultipleKeys(txID, ns, coll string, keys []string) ([][]byte, error) {
	if err := h.checkDone(); err != nil {
		return nil, err
	}

	config, err := h.collNameValidator.getCollConfig(ns, coll)
	if err != nil {
		return nil, err
	}

	staticConfig := config.GetStaticCollectionConfig()
	if staticConfig == nil {
		return nil, errors.New("invalid collection config")
	}

	value, handled, err := h.pvtDataHandler.HandleGetPrivateDataMultipleKeys(txID, ns, staticConfig, keys)
	if err != nil {
		return nil, err
	}

	if handled {
		return value, nil
	}

	return h.getPrivateDataMultipleKeys(ns, coll, keys)
}
