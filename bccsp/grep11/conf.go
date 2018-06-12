/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package grep11

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/asn1"
	"fmt"
	"hash"

	"golang.org/x/crypto/sha3"
)

type config struct {
	ellipticCurve asn1.ObjectIdentifier
	hashFunction  func() hash.Hash
	aesBitLength  int
	rsaBitLength  int

	softVerify bool
	address    string
	port       string
}

func (conf *config) setSecurityLevel(securityLevel int, hashFamily string, opts GREP11Opts) (err error) {
	switch hashFamily {
	case "SHA2":
		err = conf.setSecurityLevelSHA2(securityLevel)
	case "SHA3":
		err = conf.setSecurityLevelSHA3(securityLevel)
	default:
		err = fmt.Errorf("Hash Family not supported [%s]", hashFamily)
	}

	conf.softVerify = opts.SoftVerify
	conf.address = opts.Address
	conf.port = opts.Port
	return
}

func (conf *config) setSecurityLevelSHA2(level int) (err error) {
	switch level {
	case 256:
		conf.ellipticCurve = oidNamedCurveP256
		conf.hashFunction = sha256.New
		conf.rsaBitLength = 2048
		conf.aesBitLength = 32
	case 384:
		conf.ellipticCurve = oidNamedCurveP384
		conf.hashFunction = sha512.New384
		conf.rsaBitLength = 3072
		conf.aesBitLength = 32
	default:
		err = fmt.Errorf("Security level not supported [%d]", level)
	}
	return
}

func (conf *config) setSecurityLevelSHA3(level int) (err error) {
	switch level {
	case 256:
		conf.ellipticCurve = oidNamedCurveP256
		conf.hashFunction = sha3.New256
		conf.rsaBitLength = 2048
		conf.aesBitLength = 32
	case 384:
		conf.ellipticCurve = oidNamedCurveP384
		conf.hashFunction = sha3.New384
		conf.rsaBitLength = 3072
		conf.aesBitLength = 32
	default:
		err = fmt.Errorf("Security level not supported [%d]", level)
	}
	return
}

// PKCS11Opts contains options for the P11Factory
type GREP11Opts struct {
	// Default algorithms when not specified (Deprecated?)
	SecLevel   int    `mapstructure:"security" json:"security"`
	HashFamily string `mapstructure:"hash" json:"hash"`

	// Keystore options
	FileKeystore *FileKeystoreOpts `mapstructure:"filekeystore,omitempty" json:"filekeystore,omitempty"`

	Address string `mapstructure:"address" json:"library"`
	Port    string `mapstructure:"port" json:"libraryport"`

	SoftVerify bool `mapstructure:"softwareverify,omitempty" json:"softwareverify,omitempty"`
}

// Since currently only ECDSA operations go to PKCS11, need a keystore still
// Pluggable Keystores, could add JKS, P12, etc..
type FileKeystoreOpts struct {
	KeyStorePath string `mapstructure:"keystore" json:"keystore" yaml:"KeyStore"`
}
