/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package library

import (
	"github.com/hyperledger/fabric/extensions/handlers"
)

// loadExtension attempts to load the handler from extensions and returns true if the handler was found; false otherwise
func (r *registry) loadExtension(handlerFactory string, handlerType HandlerType, extraArgs ...string) bool {
	if handlerType == Auth {
		if f := handlers.GetAuthFilter(handlerFactory); f != nil {
			r.filters = append(r.filters, f)
			return true
		}
	} else if handlerType == Decoration {
		if d := handlers.GetDecorator(handlerFactory); d != nil {
			r.decorators = append(r.decorators, d)
			return true
		}
	}

	return false
}
