/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package csp

import (
	"crypto"
	"encoding/pem"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hyperledger/fabric/bccsp"
	"github.com/hyperledger/fabric/bccsp/factory"
	"github.com/hyperledger/fabric/bccsp/signer"
	"github.com/pkg/errors"
	gcx "github.com/zhigui-projects/gm-crypto/x509"
)

// LoadPrivateKey loads a private key from file in keystorePath
func LoadPrivateKey(keystorePath, cryptoConf string) (bccsp.Key, crypto.Signer, error) {
	var priv bccsp.Key
	var s crypto.Signer
	csp, err := GetBCCSP(keystorePath, cryptoConf)
	if err != nil {
		return nil, nil, err
	}

	walkFunc := func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, "_sk") {
			rawKey, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			block, _ := pem.Decode(rawKey)
			if block == nil {
				return errors.Errorf("%s: wrong PEM encoding", path)
			}
			priv, err = csp.KeyImport(block.Bytes, &bccsp.DefaultKeyImportOpts{Temporary: true})
			if err != nil {
				return err
			}

			s, err = signer.New(csp, priv)
			if err != nil {
				return err
			}

			return nil
		}
		return nil
	}

	err = filepath.Walk(keystorePath, walkFunc)
	if err != nil {
		return nil, nil, err
	}

	return priv, s, err
}

// GeneratePrivateKey creates a private key and stores it in keystorePath
func GeneratePrivateKey(keystorePath, cryptoConf string) (bccsp.Key,
	crypto.Signer, error) {

	csp, err := GetBCCSP(keystorePath, cryptoConf)
	if err != nil {
		return nil, nil, err
	}

	var priv bccsp.Key
	var s crypto.Signer
	// generate a key
	priv, err = csp.KeyGen(&bccsp.DefaultKeyGenOpts{Temporary: false})
	if err != nil {
		return nil, nil, err
	}
	// create a crypto.Signer
	s, err = signer.New(csp, priv)
	if err != nil {
		return nil, nil, err
	}
	return priv, s, err
}

func GetECPublicKey(priv bccsp.Key) (interface{}, error) {
	// get the public key
	pubKey, err := priv.PublicKey()
	if err != nil {
		return nil, err
	}
	// marshal to bytes
	pubKeyBytes, err := pubKey.Bytes()
	if err != nil {
		return nil, err
	}
	// unmarshal using pkix
	ecPubKey, err := gcx.GetX509().ParsePKIXPublicKey(pubKeyBytes)
	if err != nil {
		return nil, err
	}
	return ecPubKey, nil
}

func GetBCCSP(keystorePath, conf string) (bccsp.BCCSP, error) {
	groups := strings.Split(conf, "/")
	hash := strings.Split(groups[2], "-")
	secLevel, _ := strconv.Atoi(hash[1])

	opts := &factory.FactoryOpts{
		ProviderName: groups[0],
		SwOpts: &factory.SwOpts{
			Algorithm:  groups[1],
			HashFamily: hash[0],
			SecLevel:   secLevel,

			FileKeystore: &factory.FileKeystoreOpts{
				KeyStorePath: keystorePath,
			},
		},
	}
	csp, err := factory.GetBCCSPFromOpts(opts)
	if err != nil {
		return nil, err
	}

	return csp, nil
}
