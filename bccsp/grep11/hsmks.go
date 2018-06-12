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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hyperledger/fabric/bccsp"
	"github.com/hyperledger/fabric/bccsp/utils"
)

func NewHsmBasedKeyStore(path string, fallbackKS bccsp.KeyStore) (*hsmBasedKeyStore, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("Cannot find keystore directory %s", path)
	}

	ks := &hsmBasedKeyStore{}
	ks.path = path
	ks.KeyStore = fallbackKS
	return ks, nil
}

func newPin() ([]byte, error) {
	const pinLen = 8
	pinLetters := []byte("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	pin := make([]byte, pinLen)
	_, err := rand.Read(pin)
	if err != nil {
		return nil, fmt.Errorf("Failed on rand.Read() in genPin [%s]", err)
	}

	for i := 0; i < pinLen; i++ {
		index := int(pin[i])
		size := len(pinLetters)
		pin[i] = pinLetters[index%size]
	}
	return pin, nil
}

func newNonce() ([]byte, error) {
	const nonceLen = 1024
	nonce := make([]byte, nonceLen)
	_, err := rand.Read(nonce)
	if err != nil {
		return nil, fmt.Errorf("Failed on rand.Read() in getNonce [%s]", err)
	}
	return nonce, nil
}

type hsmBasedKeyStore struct {
	bccsp.KeyStore
	path string

	// Sync
	m sync.Mutex
}

func (ks *hsmBasedKeyStore) getPinAndNonce() (pin, nonce []byte, isNewPin bool, err error) {
	pinPath := ks.getPathForAlias("pin", "nonce")
	_, err = os.Stat(pinPath)
	if os.IsNotExist(err) {
		pin, err = newPin()
		if err != nil {
			return nil, nil, true, fmt.Errorf("Could not generate pin %s", err)
		}
		nonce, err = newNonce()
		if err != nil {
			return nil, nil, true, fmt.Errorf("Could not generate nonce %s", err)
		}
		logger.Debugf("Generated new pin %s and nonce", pin)
		isNewPin = true
	} else {
		raw, err := ioutil.ReadFile(pinPath)
		if err != nil {
			logger.Fatalf("Failed loading pin and nonce: [%s].", err)
		}
		block, rest := pem.Decode(raw)
		if block == nil || block.Type != "PIN" {
			return nil, nil, true, fmt.Errorf("Failed to decode PEM block containing pin")
		}
		pin = block.Bytes
		block, _ = pem.Decode(rest)
		if block == nil || block.Type != "NONCE" {
			return nil, nil, true, fmt.Errorf("Failed to decode PEM block containing pin")
		}
		nonce = block.Bytes
		isNewPin = false
		logger.Debugf("Loaded existing pin %s and nonce", pin)
	}

	return pin, nonce, isNewPin, nil
}

func (ks *hsmBasedKeyStore) storePinAndNonce(pin, nonce []byte) error {
	pinPath := ks.getPathForAlias("pin", "nonce")
	pinNnonce := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PIN",
			Bytes: pin,
		})

	pinNnonce = append(pinNnonce, pem.EncodeToMemory(
		&pem.Block{
			Type:  "NONCE",
			Bytes: nonce,
		})...)

	err := ioutil.WriteFile(pinPath, pinNnonce, 0700)
	if err != nil {
		return fmt.Errorf("Failed storing pin and nonce: [%s]", err)
	}
	return nil
}

// ReadOnly returns true if this KeyStore is read only, false otherwise.
// If ReadOnly is true then StoreKey will fail.
func (ks *hsmBasedKeyStore) ReadOnly() bool {
	return false
}

// GetKey returns a key object whose SKI is the one passed.
func (ks *hsmBasedKeyStore) GetKey(ski []byte) (k bccsp.Key, err error) {
	// Validate arguments
	if len(ski) == 0 {
		return nil, errors.New("Invalid SKI. Cannot be of zero length.")
	}

	suffix := ks.getSuffix(hex.EncodeToString(ski))

	switch suffix {
	case "sk":
		// Load the private key
		keyBlob, err := ks.loadPrivateKey(hex.EncodeToString(ski))
		if err != nil {
			logger.Debugf("Failed loading secret key [%x] [%s]", ski, err)
			break
		}

		// Load the public key
		key, err := ks.loadPublicKey(hex.EncodeToString(ski))
		if err != nil {
			return nil, fmt.Errorf("Failed loading public key [%x] [%s]", ski, err)
		}

		pubKey, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("Failed loading public key, expected type *ecdsa.PublicKey [%s]", ski)
		}

		pubKeyBlob, err := pubKeyToBlob(pubKey)
		if err != nil {
			return nil, fmt.Errorf("Failed marshaling HSM pubKeyBlob [%s]", err)
		}

		return &ecdsaPrivateKey{ski, keyBlob, &ecdsaPublicKey{ski, pubKeyBlob, pubKey}}, nil
	case "pk":
		// Load the public key
		key, err := ks.loadPublicKey(hex.EncodeToString(ski))
		if err != nil {
			return nil, fmt.Errorf("Failed loading public key [%x] [%s]", ski, err)
		}

		pubKey, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("Failed loading public key, expected type *ecdsa.PublicKey [%s]", ski)
		}

		pubKeyBlob, err := pubKeyToBlob(pubKey)
		if err != nil {
			return nil, fmt.Errorf("Failed marshaling HSM pubKeyBlob [%s]", err)
		}

		return &ecdsaPublicKey{ski, pubKeyBlob, pubKey}, nil
	}

	return ks.KeyStore.GetKey(ski)
}

// StoreKey stores the key k in this KeyStore.
// If this KeyStore is read only then the method will fail.
func (ks *hsmBasedKeyStore) StoreKey(k bccsp.Key) (err error) {
	if k == nil {
		return errors.New("Invalid key. It must be different from nil.")
	}

	switch k.(type) {
	case *ecdsaPrivateKey:
		kk := k.(*ecdsaPrivateKey)

		err = ks.storePrivateKey(hex.EncodeToString(k.SKI()), kk.keyBlob)
		if err != nil {
			return fmt.Errorf("Failed storing ECDSA private key [%s]", err)
		}

		err = ks.storePublicKey(hex.EncodeToString(k.SKI()), kk.pub.pub)
		if err != nil {
			return fmt.Errorf("Failed storing ECDSA public key [%s]", err)
		}

	case *ecdsaPublicKey:
		kk := k.(*ecdsaPublicKey)

		err = ks.storePublicKey(hex.EncodeToString(k.SKI()), kk.pub)
		if err != nil {
			return fmt.Errorf("Failed storing ECDSA public key [%s]", err)
		}

	default:
		ks.KeyStore.StoreKey(k)
	}

	return
}

func (ks *hsmBasedKeyStore) getSuffix(alias string) string {
	rc := ""
	files, _ := ioutil.ReadDir(ks.path)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), alias) {
			if strings.HasSuffix(f.Name(), "sk") {
				// Found private key
				return "sk"
			}
			if strings.HasSuffix(f.Name(), "pk") {
				// Found public key, try to find matching private key instead
				rc = "pk"
				continue
			}
			if strings.HasSuffix(f.Name(), "key") {
				// Found symmetric key
				return "key"
			}
			break
		}
	}
	return rc
}

func (ks *hsmBasedKeyStore) storePrivateKey(alias string, raw []byte) error {
	encodedKey := pem.EncodeToMemory(
		&pem.Block{
			Type:  "HSM ENCRYPTED PRIVATE KEY",
			Bytes: raw,
		})

	err := ioutil.WriteFile(ks.getPathForAlias(alias, "sk"), encodedKey, 0700)
	if err != nil {
		return fmt.Errorf("Failed storing private key [%s]: [%s]", alias, err)
	}

	return nil
}

func (ks *hsmBasedKeyStore) storePublicKey(alias string, publicKey interface{}) error {
	rawKey, err := utils.PublicKeyToPEM(publicKey, nil)
	if err != nil {
		return fmt.Errorf("Failed converting public key to PEM [%s]: [%s]", alias, err)
	}

	err = ioutil.WriteFile(ks.getPathForAlias(alias, "pk"), rawKey, 0700)
	if err != nil {
		return fmt.Errorf("Failed storing public key [%s]: [%s]", alias, err)
	}

	return nil
}

func (ks *hsmBasedKeyStore) loadPrivateKey(alias string) ([]byte, error) {
	path := ks.getPathForAlias(alias, "sk")
	logger.Debugf("Loading private key [%s] at [%s]...", alias, path)

	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Failed loading private key [%s]: [%s].", alias, err.Error())
	}

	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "HSM ENCRYPTED PRIVATE KEY" {
		return nil, fmt.Errorf("Failed to decode PEM block containing private key")
	}

	if block.Bytes == nil {
		return nil, fmt.Errorf("Found no private key blob in file")
	}

	return block.Bytes, nil
}

func (ks *hsmBasedKeyStore) loadPublicKey(alias string) (interface{}, error) {
	path := ks.getPathForAlias(alias, "pk")
	logger.Debugf("Loading public key [%s] at [%s]...", alias, path)

	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Failed loading public key [%s]: [%s].", alias, err.Error())
	}

	publicKey, err := utils.PEMtoPublicKey(raw, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed parsing public key [%s]: [%s].", alias, err.Error())
	}

	return publicKey, nil
}

func (ks *hsmBasedKeyStore) getPathForAlias(alias, suffix string) string {
	return filepath.Join(ks.path, alias+"_"+suffix)
}

type EckeyIdentASN struct {
	KeyType asn1.ObjectIdentifier
	Curve   asn1.ObjectIdentifier
}

type PubKeyASN struct {
	Ident EckeyIdentASN
	Point asn1.BitString
}

func blobToPubKey(pubKey []byte, curve asn1.ObjectIdentifier) ([]byte, *ecdsa.PublicKey, error) {
	nistCurve := namedCurveFromOID(curve)
	if curve == nil {
		return nil, nil, fmt.Errorf("Cound not recognize Curve from OID")
	}

	decode := &PubKeyASN{}
	_, err := asn1.Unmarshal(pubKey, decode)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed Unmarshaling Public Key [%s]", err)
	}

	hash := sha256.Sum256(decode.Point.Bytes)
	ski := hash[:]

	x, y := elliptic.Unmarshal(nistCurve, decode.Point.Bytes)
	if x == nil {
		return nil, nil, fmt.Errorf("Failed Unmarshaling Public Key..\n%s", hex.Dump(decode.Point.Bytes))
	}

	return ski, &ecdsa.PublicKey{Curve: nistCurve, X: x, Y: y}, nil
}

func pubKeyToBlob(pubKey *ecdsa.PublicKey) ([]byte, error) {
	if pubKey == nil {
		return nil, fmt.Errorf("Value of Public Key was nil")
	}

	oid, ok := oidFromNamedCurve(pubKey.Curve)
	point := elliptic.Marshal(pubKey.Curve, pubKey.X, pubKey.Y)
	if !ok {
		return nil, fmt.Errorf("Curve not recognized")
	}

	encode := &PubKeyASN{
		Ident: EckeyIdentASN{
			KeyType: asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}, //ecPublicKey
			Curve:   oid,
		},
		Point: asn1.BitString{
			Bytes:     point,
			BitLength: len(point) * 8,
		},
	}

	return asn1.Marshal(*encode)
}
