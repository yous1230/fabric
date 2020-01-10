/*
Copyright Zhigui.com. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sw

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSM4EncryptSM4Decrypt encrypts using SM4Encrypt and decrypts using SM4Decrypt.
func TestSM4EncryptSM4Decrypt(t *testing.T) {
	t.Parallel()

	key := make([]byte, 16)
	_, _ = rand.Reader.Read(key)

	var ptext = []byte("a message with arbitrary length (42 bytes)")

	encrypted, encErr := SM4Encrypt(key, ptext)
	if encErr != nil {
		t.Fatalf("Error encrypting '%s': %s", ptext, encErr)
	}

	decrypted, dErr := SM4Decrypt(key, encrypted)
	if dErr != nil {
		t.Fatalf("Error decrypting the encrypted '%s': %v", ptext, dErr)
	}

	if string(ptext[:]) != string(decrypted[:]) {
		t.Fatal("Decrypt( Encrypt( ptext ) ) != ptext: Ciphertext decryption with the same key must result in the original plaintext!")
	}
}

func TestSM4EncryptorDecrypt(t *testing.T) {
	t.Parallel()

	key := make([]byte, 16)
	_, _ = rand.Reader.Read(key)

	k := &gmsm4PrivateKey{privKey: key, exportable: false}

	msg := []byte("Hello World")
	encryptor := &gmsm4Encryptor{}

	ct, err := encryptor.Encrypt(k, msg, nil)
	assert.NoError(t, err)

	decryptor := &gmsm4Decryptor{}

	msg2, err := decryptor.Decrypt(k, ct, nil)
	assert.NoError(t, err)
	assert.Equal(t, msg, msg2)
}
