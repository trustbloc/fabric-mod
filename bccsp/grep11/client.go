/*
Copyright IBM Corp. 2017 All Rights Reserved.

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
	"crypto/elliptic"
	"encoding/asn1"
	"fmt"
	"math/big"

	pb "github.com/hyperledger/fabric/bccsp/grep11/protos"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RFC 5480, 2.1.1.1. Named Curve
//
// secp224r1 OBJECT IDENTIFIER ::= {
//   iso(1) identified-organization(3) certicom(132) curve(0) 33 }
//
// secp256r1 OBJECT IDENTIFIER ::= {
//   iso(1) member-body(2) us(840) ansi-X9-62(10045) curves(3)
//   prime(1) 7 }
//
// secp384r1 OBJECT IDENTIFIER ::= {
//   iso(1) identified-organization(3) certicom(132) curve(0) 34 }
//
// secp521r1 OBJECT IDENTIFIER ::= {
//   iso(1) identified-organization(3) certicom(132) curve(0) 35 }
//
var (
	oidNamedCurveP224 = asn1.ObjectIdentifier{1, 3, 132, 0, 33}
	oidNamedCurveP256 = asn1.ObjectIdentifier{1, 2, 840, 10045, 3, 1, 7}
	oidNamedCurveP384 = asn1.ObjectIdentifier{1, 3, 132, 0, 34}
	oidNamedCurveP521 = asn1.ObjectIdentifier{1, 3, 132, 0, 35}
)

func namedCurveFromOID(oid asn1.ObjectIdentifier) elliptic.Curve {
	switch {
	case oid.Equal(oidNamedCurveP224):
		return elliptic.P224()
	case oid.Equal(oidNamedCurveP256):
		return elliptic.P256()
	case oid.Equal(oidNamedCurveP384):
		return elliptic.P384()
	case oid.Equal(oidNamedCurveP521):
		return elliptic.P521()
	}
	return nil
}

func oidFromNamedCurve(curve elliptic.Curve) (asn1.ObjectIdentifier, bool) {
	switch curve {
	case elliptic.P224():
		return oidNamedCurveP224, true
	case elliptic.P256():
		return oidNamedCurveP256, true
	case elliptic.P384():
		return oidNamedCurveP384, true
	case elliptic.P521():
		return oidNamedCurveP521, true
	}

	return nil, false
}

func (csp *impl) reLoad(grpcCall func() error) error {
	err := grpcCall()
	if err != nil {
		grpcStatus, ok := status.FromError(err)
		if ok {
			switch grpcStatus.Code() {
			case codes.Unavailable, codes.FailedPrecondition:
				logger.Debugf("GRPC Error received, reconnecting: [%s]", grpcStatus.Code().String())
				err = csp.connectSession()
				if err == nil {
					err = grpcCall()
				}
			}
		}
	}
	return err
}

func (csp *impl) generateECKey(curve asn1.ObjectIdentifier, ephemeral bool) (*ecdsaPrivateKey, error) {
	marshaledOID, err := asn1.Marshal(curve)
	if err != nil {
		return nil, fmt.Errorf("Could not marshal OID [%s]", err.Error())
	}

	var k *pb.GenerateStatus
	err = csp.reLoad(func() error {
		var err error
		k, err = csp.grepClient.GenerateECKey(context.Background(), &pb.GenerateInfo{Oid: marshaledOID})
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("Could not remote-generate PKCS11 library [%s]\n Remote Response: <%+v>", err, k)
	}
	if k.Error != "" {
		return nil, fmt.Errorf("Remote Generate call reports error: %s", k.Error)
	}

	ski, pubGoKey, err := blobToPubKey(k.PubKey, curve)
	if err != nil {
		return nil, fmt.Errorf("Failed Unmarshaling Public Key [%s]", err)
	}

	/* VP DELETE Verifying pub key generation from soft key
	ioutil.WriteFile("/tmp/pub.asn1", k.PubKey, 0644)
	checkBlob, err := pubKeyToBlob(pubGoKey)
	if err != nil {
		return nil, fmt.Errorf("Well this is strange! [%s]", err)
	}

	if bytes.Equal(k.PubKey, checkBlob) {
		logger.Fatalf("VP>>>>>>>>>>>>>>>>>>>>>>>> That was too easy?")
	} else {
		logger.Fatalf("Keys mismatch\nExpected:\n%s\nGenerated:\n%s", hex.Dump(k.PubKey), hex.Dump(checkBlob))
	}
	//endDELETE */

	key := &ecdsaPrivateKey{ski, k.PrivKey, &ecdsaPublicKey{ski, k.PubKey, pubGoKey}}
	return key, nil
}

func (csp *impl) signP11ECDSA(keyBlob []byte, msg []byte) (R, S *big.Int, err error) {
	var sig *pb.SignStatus
	err = csp.reLoad(func() error {
		var err error
		sig, err = csp.grepClient.SignP11ECDSA(context.Background(), &pb.SignInfo{PrivKey: keyBlob, Hash: msg})
		return err
	})

	if err != nil {
		return nil, nil, fmt.Errorf("Could not remote-sign PKCS11 library [%s]\n Remote Response: <%s>", err, sig)
	}
	if sig.Error != "" {
		return nil, nil, fmt.Errorf("Remote Sign call reports error: %s", sig.Error)
	}

	R = new(big.Int)
	S = new(big.Int)
	R.SetBytes(sig.Sig[0 : len(sig.Sig)/2])
	S.SetBytes(sig.Sig[len(sig.Sig)/2:])

	return R, S, nil
}

func (csp *impl) verifyP11ECDSA(keyBlob []byte, msg []byte, R, S *big.Int, byteSize int) (valid bool, err error) {
	// TODO: Uncomment when HSM Verify is supported
	//r := R.Bytes()
	//s := S.Bytes()
	//
	//// Pad front of R and S with Zeroes if needed
	//sig := make([]byte, 2*byteSize)
	//copy(sig[byteSize-len(r):byteSize], r)
	//copy(sig[2*byteSize-len(s):], s)
	//
	//val, err := csp.grepClient.VerifyP11ECDSA(context.Background(), &pb.VerifyInfo{keyBlob, msg, sig})
	//if err != nil {
	//	return false, fmt.Errorf("Could not remote-verify PKCS11 library [%s]\n Remote Response: <%+v>", err, val)
	//}
	//if val.Error != "" {
	//	return false, fmt.Errorf("Remote Verify call reports error: %s", val.Error)
	//}
	//
	//return val.Valid, nil
	return false, fmt.Errorf("Remote Verify is currently not supported.")
}
