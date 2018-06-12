// +build pkcs11

/*
Copyright IBM Corp. All Rights Reserved.
SPDX-License-Identifier: Apache-2.0
*/

package factory

import (
	"errors"
	"fmt"

	"github.com/hyperledger/fabric/bccsp"
	"github.com/hyperledger/fabric/bccsp/grep11"
	"github.com/hyperledger/fabric/bccsp/sw"
)

const (
	// GREP11BasedFactoryName is the name of the factory of the hsm-based BCCSP implementation
	GREP11BasedFactoryName = "GREP11"
)

// GREP11Factory is the factory of the HSM-based BCCSP.
type GREP11Factory struct{}

// Name returns the name of this factory
func (f *GREP11Factory) Name() string {
	return GREP11BasedFactoryName
}

// Get returns an instance of BCCSP using Opts.
func (f *GREP11Factory) Get(config *FactoryOpts) (bccsp.BCCSP, error) {
	// Validate arguments
	if config == nil || config.Grep11Opts == nil {
		return nil, errors.New("Invalid config. It must not be nil.")
	}

	p11Opts := config.Grep11Opts

	var ks bccsp.KeyStore
	if p11Opts.FileKeystore != nil {
		fks, err := sw.NewFileBasedKeyStore(nil, p11Opts.FileKeystore.KeyStorePath, false)
		if err != nil {
			return nil, fmt.Errorf("Failed to initialize software key store: %s", err)
		}
		ks = fks
	} else {
		// Default to DummyKeystore
		ks = sw.NewDummyKeyStore()
	}
	return grep11.New(*p11Opts, ks)
}
