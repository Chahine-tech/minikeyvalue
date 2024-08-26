package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
)

// EncryptData encrypts the given data using the provided key.
func EncryptData(data []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	log.Printf("EncryptData: nonce size: %d, data size: %d, ciphertext size: %d", len(nonce), len(data), len(ciphertext))
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptData decrypts the given encrypted data using the provided key.
func DecryptData(encryptedData string, key []byte) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		log.Printf("DecryptData: malformed ciphertext, data size: %d, nonce size: %d", len(data), nonceSize)
		return nil, errors.New("malformed ciphertext")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		log.Printf("DecryptData: message authentication failed, nonce size: %d, ciphertext size: %d", len(nonce), len(ciphertext))
		return nil, err
	}

	log.Printf("DecryptData: Data decrypted successfully, plaintext length: %d", len(plaintext))
	return plaintext, nil
}

// RotateEncryptionKey rotates the encryption key for the KeyValueStore.
func (kv *KeyValueStore) RotateEncryptionKey(newEncryptionKey []byte) error {
	kv.Lock()
	defer kv.Unlock()

	log.Println("Starting key rotation...")

	// Save current data to bytes
	data, err := kv.saveToBytes()
	if err != nil {
		log.Println("Failed to save current data:", err)
		return fmt.Errorf("failed to save current data: %v", err)
	}
	log.Println("Current data saved successfully.")

	// Décryptage des données avec l'ancienne clé
	oldEncryptionKey := kv.encryptionKey
	decryptedData, err := DecryptData(string(data), oldEncryptionKey)
	if err != nil {
		log.Println("Failed to decrypt data with old key:", err)
		return fmt.Errorf("failed to decrypt data with old key: %v", err)
	}
	log.Println("Data decrypted with old key.")

	// Encryptage avec la nouvelle clé
	kv.encryptionKey = newEncryptionKey
	encryptedData, err := EncryptData(decryptedData, kv.encryptionKey)
	if err != nil {
		log.Println("Failed to encrypt data with new key:", err)
		kv.encryptionKey = oldEncryptionKey // Revert the key on error
		return fmt.Errorf("failed to encrypt data with new key: %v", err)
	}
	log.Println("Data encrypted with new key.")

	// Load the encrypted data back into the KeyValueStore
	if err := kv.loadFromBytes([]byte(encryptedData)); err != nil {
		log.Println("Failed to load data with new encryption:", err)
		kv.encryptionKey = oldEncryptionKey // Revert the key on error
		return fmt.Errorf("failed to load data with new encryption: %v", err)
	}
	log.Println("Data loaded with new encryption.")

	// Persist the new encrypted data
	if err := kv.save(); err != nil {
		log.Println("Failed to save data with new encryption key:", err)
		kv.encryptionKey = oldEncryptionKey // Revert the key on error
		return fmt.Errorf("failed to save data with new encryption key: %v", err)
	}
	log.Println("Data saved with new encryption key. Key rotation completed successfully.")

	return nil
}

// saveToBytes serializes the in-memory data to a byte slice.
func (kv *KeyValueStore) saveToBytes() ([]byte, error) {
	data, err := json.Marshal(kv.data)
	if err != nil {
		return nil, fmt.Errorf("error marshalling data: %v", err)
	}

	compressedData, err := CompressData(data)
	if err != nil {
		return nil, fmt.Errorf("error compressing data: %v", err)
	}

	if len(kv.encryptionKey) > 0 {
		encryptedData, err := EncryptData(compressedData, kv.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("error encrypting data: %v", err)
		}
		return []byte(encryptedData), nil
	}

	return compressedData, nil
}

// loadFromBytes loads the data from a byte slice into the in-memory data structure.
func (kv *KeyValueStore) loadFromBytes(data []byte) error {
	var err error
	var decompressedData []byte

	if len(kv.encryptionKey) > 0 {
		data, err = DecryptData(string(data), kv.encryptionKey)
		if err != nil {
			return fmt.Errorf("error decrypting data: %v", err)
		}
	}

	decompressedData, err = DecompressData(data)
	if err != nil {
		return fmt.Errorf("error decompressing data: %v", err)
	}

	if err := json.Unmarshal(decompressedData, &kv.data); err != nil {
		return fmt.Errorf("error unmarshalling data: %v", err)
	}

	return nil
}
