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
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"time"

	"github.com/hyperledger/fabric/bccsp"
	pb "github.com/hyperledger/fabric/bccsp/grep11/protos"
	"github.com/hyperledger/fabric/bccsp/sw"
	"github.com/hyperledger/fabric/bccsp/utils"
	"github.com/hyperledger/fabric/common/flogging"
	"google.golang.org/grpc"
)

var (
	logger           = flogging.MustGetLogger("bccsp_ep11")
	sessionCacheSize = 10
)

// New returns a new instance of the software-based BCCSP
// set at the passed security level, hash family and KeyStore.
func New(opts GREP11Opts, fallbackKS bccsp.KeyStore) (bccsp.BCCSP, error) {
	// Init config
	conf := &config{}
	err := conf.setSecurityLevel(opts.SecLevel, opts.HashFamily, opts)
	if err != nil {
		return nil, fmt.Errorf("Failed initializing configuration [%s]", err)
	}

	// Note: If the fallbackKS is nil, the sw.New function will catch the error
	swCSP, err := sw.NewWithParams(opts.SecLevel, opts.HashFamily, fallbackKS)
	if err != nil {
		return nil, fmt.Errorf("Failed initializing fallback SW BCCSP [%s]", err)
	}

	if opts.FileKeystore == nil {
		return nil, fmt.Errorf("FileKeystore is required to use GREP11 CSP")
	}

	keyStore, err := NewHsmBasedKeyStore(opts.FileKeystore.KeyStorePath, fallbackKS)
	if err != nil {
		return nil, fmt.Errorf("Failed initializing HSMBasedKeyStore [%s]", err)
	}

	csp := &impl{
		BCCSP: swCSP,
		conf:  conf,
		ks:    keyStore,
	}
	err = csp.connectSession()
	if err != nil {
		return nil, fmt.Errorf("Failed connecting to GREP11 Manager [%s]", err)
	}

	return csp, nil
}

type impl struct {
	bccsp.BCCSP

	conf *config
	ks   bccsp.KeyStore

	grepClient       pb.Grep11Client
	grepManager      pb.Grep11ManagerClient
	clientConnection *grpc.ClientConn
}

func (csp *impl) connectSession() error {
	logger.Debugf("Connecting to GREP11 Master %s:%s", csp.conf.address, csp.conf.port)

	// Setup timeout context for manager connection
	mgrCtx, cancelMgrConn := context.WithTimeout(context.Background(), time.Second*60)
	defer cancelMgrConn()

	mgrConn, err := grpc.DialContext(mgrCtx, csp.conf.address+":"+csp.conf.port, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return fmt.Errorf("Failed connecting to GREP11 manager at %s:%s [%s]", csp.conf.address, csp.conf.port, err)
	}

	// Close the manager TCP connection to the GREP11 Manager after
	// connecting to its GREP11 Server service
	defer mgrConn.Close()

	csp.grepManager = pb.NewGrep11ManagerClient(mgrConn)

	pin, nonce, isNewPin, err := csp.ks.(*hsmBasedKeyStore).getPinAndNonce()
	if err != nil {
		return fmt.Errorf("Failed generating PIN and Nonce for the EP11 session [%s]", err)
	}

	if !isNewPin && len(pin) == 0 && len(nonce) == 0 {
		logger.Warningf("Starting GREP11 BCCSP without a session! Using Domain Master key to encrypt/decrypt key material.")
		//TODO: We could attempt to log in with a new session at this point
		//      if that were to succeed, re-wrap keys with new session
		//      this might also be a place to place generic 're-wrap logic' (i.e. if Master Key changed
		//      when container got moved to different LPAR)
	}

	r, err := csp.grepManager.Load(context.Background(), &pb.LoadInfo{Pin: pin, Nonce: nonce})
	if err != nil {
		return fmt.Errorf("Could not remote-load EP11 library [%s]\n Remote Response: <%+v>", err, r)
	}
	if r.Error != "" {
		return fmt.Errorf("Remote Load call reports error: %s", r.Error)
	}

	if r.Session == false {
		// Ran out of sessions!!
		if !isNewPin && len(pin) != 0 && len(nonce) != 0 {
			// This is bad! Existing keys are inaccessible.
			return fmt.Errorf("Failed to log in into EP11 session. Crypto material inaccessible.")
		}

		// Carry on with reduced container isolation.
		logger.Warningf("ep11server ran out of sessions!! Using Domain Master key to encrypt/decrypt key material.")
		pin = nil
		nonce = nil
	}

	if isNewPin {
		err = csp.ks.(*hsmBasedKeyStore).storePinAndNonce(pin, nonce)
		if err != nil {
			return fmt.Errorf("Failed storing PIN and nonce [%s]", err)
		}
	}

	// Setup timeout context for server connection
	srvrCtx, cancelSrvrConn := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelSrvrConn()

	srvrConn, err := grpc.DialContext(srvrCtx, r.Address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return fmt.Errorf("Failed connecting to GREP11 dedicated connection at %s [%s]", r.Address, err)
	}

	logger.Infof("Connected to a dedicated crypto Server connection at %s", r.Address)

	csp.grepClient = pb.NewGrep11Client(srvrConn)
	csp.clientConnection = srvrConn
	return nil
}

// KeyGen generates a key using opts.
func (csp *impl) KeyGen(opts bccsp.KeyGenOpts) (k bccsp.Key, err error) {
	// Validate arguments
	if opts == nil {
		return nil, errors.New("Invalid Opts parameter. It must not be nil.")
	}

	// Parse algorithm
	switch opts.(type) {
	case *bccsp.ECDSAKeyGenOpts:
		k, err = csp.generateECKey(csp.conf.ellipticCurve, opts.Ephemeral())
		if err != nil {
			return nil, fmt.Errorf("Failed generating ECDSA key [%s]", err)
		}

	case *bccsp.ECDSAP256KeyGenOpts:
		k, err = csp.generateECKey(oidNamedCurveP256, opts.Ephemeral())
		if err != nil {
			return nil, fmt.Errorf("Failed generating ECDSA P256 key [%s]", err)
		}

	case *bccsp.ECDSAP384KeyGenOpts:
		k, err = csp.generateECKey(oidNamedCurveP384, opts.Ephemeral())
		if err != nil {
			return nil, fmt.Errorf("Failed generating ECDSA P384 key [%s]", err)
		}

	default:
		return csp.BCCSP.KeyGen(opts)
	}

	if !opts.Ephemeral() {
		err := csp.ks.StoreKey(k)
		if err != nil {
			return nil, fmt.Errorf("Failed storing key [%s]", err)
		}
	}

	return k, nil
}

// KeyDeriv derives a key from k using opts.
// The opts argument should be appropriate for the primitive used.
func (csp *impl) KeyDeriv(k bccsp.Key, opts bccsp.KeyDerivOpts) (dk bccsp.Key, err error) {
	// Validate arguments
	if k == nil {
		return nil, errors.New("Invalid Key. It must not be nil.")
	}

	// Derive key
	switch k.(type) {
	case *ecdsaPrivateKey:
		return nil, fmt.Errorf("Key Derrivation not implemented with HSM Private keys yet")

	default:
		return csp.BCCSP.KeyDeriv(k, opts)

	}
}

// KeyImport imports a key from its raw representation using opts.
// The opts argument should be appropriate for the primitive used.
func (csp *impl) KeyImport(raw interface{}, opts bccsp.KeyImportOpts) (k bccsp.Key, err error) {
	// Validate arguments
	if raw == nil {
		return nil, errors.New("Invalid raw. Cannot be nil.")
	}

	if opts == nil {
		return nil, errors.New("Invalid Opts parameter. It must not be nil.")
	}

	swK, err := csp.BCCSP.KeyImport(raw, opts)
	if err != nil {
		return nil, err
	}

	if swK.Symmetric() {
		// No support for symmetric keys yet, use clear keys for now
		return swK, nil
	}

	if swK.Private() {
		return nil, errors.New("Importing Private Key into GREP11 provider is not allowed.")
	}

	// Must be public key, see if its an ECDSA key
	pubKeyBytes, err := swK.Bytes()
	if err != nil {
		return nil, fmt.Errorf("Failed marshalling public key [%s]", err)
	}

	pk, err := utils.DERToPublicKey(pubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("Failed marshalling der to public key [%s]", err)
	}

	switch k := pk.(type) {
	case *ecdsa.PublicKey:
		if k == nil {
			return nil, errors.New("Invalid ecdsa public key. It must be different from nil.")
		}

		pubKeyBlob, err := pubKeyToBlob(k)
		if err != nil {
			return nil, fmt.Errorf("Failed marshaling HSM pubKeyBlob [%s]", err)
		}
		return &ecdsaPublicKey{swK.SKI(), pubKeyBlob, k}, nil

	default:
		return swK, nil
	}
}

// GetKey returns the key this CSP associates to
// the Subject Key Identifier ski.
func (csp *impl) GetKey(ski []byte) (k bccsp.Key, err error) {
	return csp.ks.GetKey(ski)
}

// Sign signs digest using key k.
// The opts argument should be appropriate for the primitive used.
//
// Note that when a signature of a hash of a larger message is needed,
// the caller is responsible for hashing the larger message and passing
// the hash (as digest).
func (csp *impl) Sign(k bccsp.Key, digest []byte, opts bccsp.SignerOpts) (signature []byte, err error) {
	// Validate arguments
	if k == nil {
		return nil, errors.New("Invalid Key. It must not be nil.")
	}
	if len(digest) == 0 {
		return nil, errors.New("Invalid digest. Cannot be empty.")
	}

	// Check key type
	switch k.(type) {
	case *ecdsaPrivateKey:
		return csp.signECDSA(*k.(*ecdsaPrivateKey), digest, opts)
	case *ecdsaPublicKey:
		return nil, errors.New("Cannot sign with a grep11.ecdsaPublicKey")
	default:
		return csp.BCCSP.Sign(k, digest, opts)
	}
}

// Verify verifies signature against key k and digest
func (csp *impl) Verify(k bccsp.Key, signature, digest []byte, opts bccsp.SignerOpts) (valid bool, err error) {
	// Validate arguments
	if k == nil {
		return false, errors.New("Invalid Key. It must not be nil.")
	}
	if len(signature) == 0 {
		return false, errors.New("Invalid signature. Cannot be empty.")
	}
	if len(digest) == 0 {
		return false, errors.New("Invalid digest. Cannot be empty.")
	}

	// Check key type
	switch k.(type) {
	case *ecdsaPrivateKey:
		return csp.verifyECDSA(*k.(*ecdsaPrivateKey).pub, signature, digest, opts)
	case *ecdsaPublicKey:
		return csp.verifyECDSA(*k.(*ecdsaPublicKey), signature, digest, opts)
	default:
		return csp.BCCSP.Verify(k, signature, digest, opts)
	}
}

// Encrypt encrypts plaintext using key k.
// The opts argument should be appropriate for the primitive used.
func (csp *impl) Encrypt(k bccsp.Key, plaintext []byte, opts bccsp.EncrypterOpts) (ciphertext []byte, err error) {
	// TODO: Add PKCS11 support for encryption, when fabric starts requiring it
	return csp.BCCSP.Encrypt(k, plaintext, opts)
}

// Decrypt decrypts ciphertext using key k.
// The opts argument should be appropriate for the primitive used.
func (csp *impl) Decrypt(k bccsp.Key, ciphertext []byte, opts bccsp.DecrypterOpts) (plaintext []byte, err error) {
	return csp.BCCSP.Decrypt(k, ciphertext, opts)
}
