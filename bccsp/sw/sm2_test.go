/*
Copyright Zhigui.com. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sw

import (
	"crypto/elliptic"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zhigui-projects/gmsm/sm2"
	"github.com/zhigui-projects/gmsm/sm3"
)

func TestSignSM2BadParameter(t *testing.T) {
	//t.Parallel()
	// Generate a key
	lowLevelKey, err := sm2.GenerateKey()
	assert.NoError(t, err)

	// Induce an error on the underlying ecdsa algorithm
	msg := []byte("hello world")
	oldN := lowLevelKey.Params().N
	defer func() { lowLevelKey.Params().N = oldN }()
	lowLevelKey.Params().N = big.NewInt(0)
	_, err = signGMSM2(lowLevelKey, msg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "zero parameter")
	lowLevelKey.Params().N = oldN
}

func TestVerifySM2(t *testing.T) {
	t.Parallel()
	// Generate a key
	lowLevelKey, err := sm2.GenerateKey()
	assert.NoError(t, err)

	msg := []byte("hello world1")
	sigma, err := signGMSM2(lowLevelKey, msg, nil)
	assert.NoError(t, err)

	valid, err := verifyGMSM2(&lowLevelKey.PublicKey, sigma, msg, nil)
	assert.NoError(t, err)
	assert.True(t, valid)

	valid, err = verifyGMSM2(&lowLevelKey.PublicKey, nil, msg, nil)
	assert.NoError(t, err)
	assert.False(t, valid)

}

func TestSM2SignerSign(t *testing.T) {
	t.Parallel()

	signer := &gmsm2Signer{}
	verifierPrivateKey := &gmsm2PrivateKeyVerifier{}
	verifierPublicKey := &gmsm2PublicKeyKeyVerifier{}

	// Generate a key
	lowLevelKey, err := sm2.GenerateKey()
	assert.NoError(t, err)
	k := &gmsm2PrivateKey{lowLevelKey}
	pk, err := k.PublicKey()
	assert.NoError(t, err)

	// Sign
	msg := []byte("Hello World2")
	sigma, err := signer.Sign(k, msg, nil)
	assert.NoError(t, err)
	assert.NotNil(t, sigma)

	// Verify
	valid, err := verifyGMSM2(&lowLevelKey.PublicKey, sigma, msg, nil)
	assert.NoError(t, err)
	assert.True(t, valid)

	valid, err = verifierPrivateKey.Verify(k, sigma, msg, nil)
	assert.NoError(t, err)
	assert.True(t, valid)

	valid, err = verifierPublicKey.Verify(pk, sigma, msg, nil)
	assert.NoError(t, err)
	assert.True(t, valid)
}

func TestSM2PrivateKey(t *testing.T) {
	t.Parallel()

	lowLevelKey, err := sm2.GenerateKey()
	assert.NoError(t, err)
	k := &gmsm2PrivateKey{lowLevelKey}

	assert.False(t, k.Symmetric())
	assert.True(t, k.Private())

	bytes, err := k.Bytes()
	assert.NoError(t, err)
	assert.NotNil(t, bytes)

	k.privKey = nil
	ski := k.SKI()
	assert.Nil(t, ski)

	k.privKey = lowLevelKey
	ski = k.SKI()
	raw := elliptic.Marshal(k.privKey.Curve, k.privKey.PublicKey.X, k.privKey.PublicKey.Y)
	hash := sm3.New()
	hash.Write(raw)
	ski2 := hash.Sum(nil)
	assert.Equal(t, ski2, ski, "SKI is not computed in the right way.")

	pk, err := k.PublicKey()
	assert.NoError(t, err)
	assert.NotNil(t, pk)
	sm2PK, ok := pk.(*gmsm2PublicKey)
	assert.True(t, ok)
	assert.Equal(t, &lowLevelKey.PublicKey, sm2PK.pubKey)
}

func TestSM2PublicKey(t *testing.T) {
	t.Parallel()

	lowLevelKey, err := sm2.GenerateKey()
	assert.NoError(t, err)
	k := &gmsm2PublicKey{&lowLevelKey.PublicKey}

	assert.False(t, k.Symmetric())
	assert.False(t, k.Private())

	k.pubKey = nil
	ski := k.SKI()
	assert.Nil(t, ski)

	k.pubKey = &lowLevelKey.PublicKey
	ski = k.SKI()
	raw := elliptic.Marshal(k.pubKey.Curve, k.pubKey.X, k.pubKey.Y)
	hash := sm3.New()
	hash.Write(raw)
	ski2 := hash.Sum(nil)
	assert.Equal(t, ski, ski2, "SKI is not computed in the right way.")

	pk, err := k.PublicKey()
	assert.NoError(t, err)
	assert.Equal(t, k, pk)

	bytes, err := k.Bytes()
	assert.NoError(t, err)
	bytes2, err := sm2.MarshalSm2PublicKey(k.pubKey)
	assert.Equal(t, bytes2, bytes, "bytes are not computed in the right way.")
}
