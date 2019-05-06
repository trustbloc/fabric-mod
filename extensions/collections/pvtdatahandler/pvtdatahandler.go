/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package pvtdatahandler

import (
	storeapi "github.com/hyperledger/fabric/extensions/collections/api/store"
	"github.com/hyperledger/fabric/protos/common"
)

// Handler handles the retrieval of extensions-defined collection types
type Handler struct {
}

// New returns a new Handler
func New(channelID string, collDataProvider storeapi.Provider) *Handler {
	return &Handler{}
}

// HandleGetPrivateData if the collection is one of the custom extensions collections then the private data is returned
func (h *Handler) HandleGetPrivateData(txID, ns string, config *common.StaticCollectionConfig, key string) ([]byte, bool, error) {
	return nil, false, nil
}

// HandleGetPrivateDataMultipleKeys if the collection is one of the custom extensions collections then the private data is returned
func (h *Handler) HandleGetPrivateDataMultipleKeys(txID, ns string, config *common.StaticCollectionConfig, keys []string) ([][]byte, bool, error) {
	return nil, false, nil
}
