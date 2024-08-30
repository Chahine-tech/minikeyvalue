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
func EncryptData(data []byte, key []byte) ([]byte, error) {
	log.Println("EncryptData: Entered function")
	block, err := aes.NewCipher(key)
	if err != nil {
		log.Println("EncryptData: Error creating new cipher:", err)
		return nil, err
	}

	log.Println("EncryptData: Creating GCM block")
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Println("EncryptData: Error creating GCM:", err)
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	log.Println("EncryptData: Reading nonce")
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		log.Println("EncryptData: Error reading random nonce:", err)
		return nil, err
	}

	log.Println("EncryptData: Sealing data")
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	log.Printf("EncryptData: nonce size: %d, data size: %d, ciphertext size: %d", len(nonce), len(data), len(ciphertext))

	return ciphertext, nil
}

// DecryptData decrypts the given encrypted data using the provided key.
func DecryptData(encryptedData []byte, key []byte) ([]byte, error) {
	log.Println("DecryptData: Entered function")

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Println("DecryptData: Error creating new cipher:", err)
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Println("DecryptData: Error creating GCM:", err)
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		log.Println("DecryptData: Malformed ciphertext")
		return nil, errors.New("malformed ciphertext")
	}

	nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		log.Println("DecryptData: Message authentication failed")
		return nil, err
	}

	log.Println("DecryptData: Data decrypted successfully")
	return plaintext, nil
}

// RotateEncryptionKey rotates the encryption key for the KeyValueStore.
func (kv *KeyValueStore) RotateEncryptionKey(newEncryptionKey []byte) error {
	log.Println("Starting key rotation...")

	log.Println("RotateEncryptionKey: Saving current data to bytes")
	data, err := kv.saveToBytes()
	if err != nil {
		log.Println("Failed to save current data:", err)
		return fmt.Errorf("failed to save current data: %v", err)
	}
	log.Println("Current data saved successfully.")

	oldEncryptionKey := kv.encryptionKey

	log.Println("RotateEncryptionKey: Decrypting data with old key")
	decryptedData, err := DecryptData(data, oldEncryptionKey)
	if err != nil {
		log.Println("Failed to decrypt data with old key:", err)
		return fmt.Errorf("failed to decrypt data with old key: %v", err)
	}
	log.Println("Data decrypted with old key.")

	kv.encryptionKey = newEncryptionKey

	log.Println("RotateEncryptionKey: Encrypting data with new key")
	encryptedData, err := EncryptData(decryptedData, kv.encryptionKey)
	if err != nil {
		log.Println("Failed to encrypt data with new key:", err)
		kv.encryptionKey = oldEncryptionKey
		return fmt.Errorf("failed to encrypt data with new key: %v", err)
	}
	log.Println("Data encrypted with new key.")

	// Base64 encode the encrypted data
	encodedData := base64.StdEncoding.EncodeToString(encryptedData)

	log.Println("RotateEncryptionKey: Loading encrypted data into KeyValueStore")
	if err := kv.loadFromBytes([]byte(encodedData)); err != nil {
		log.Println("Failed to load data with new encryption:", err)
		kv.encryptionKey = oldEncryptionKey
		return fmt.Errorf("failed to load data with new encryption: %v", err)
	}
	log.Println("Data loaded with new encryption.")

	log.Println("RotateEncryptionKey: Persisting the new encrypted data")
	if err := kv.save(); err != nil {
		log.Println("Failed to save data with new encryption key:", err)
		kv.encryptionKey = oldEncryptionKey
		return fmt.Errorf("failed to save data with new encryption key: %v", err)
	}
	log.Println("Data saved with new encryption key. Key rotation completed successfully.")

	return nil
}

// saveToBytes serializes the in-memory data to a byte slice.
func (kv *KeyValueStore) saveToBytes() ([]byte, error) {
	kv.RLock()
	defer kv.RUnlock()

	log.Println("saveToBytes: Marshal data")
	data, err := json.Marshal(kv.data)
	if err != nil {
		log.Println("saveToBytes: Error marshalling data:", err)
		return nil, fmt.Errorf("error marshalling data: %v", err)
	}

	log.Println("saveToBytes: Compressing data")
	compressedData, err := CompressData(data)
	if err != nil {
		log.Println("saveToBytes: Error compressing data:", err)
		return nil, fmt.Errorf("error compressing data: %v", err)
	}

	if len(kv.encryptionKey) > 0 {
		log.Println("saveToBytes: Encrypting data")
		encryptedData, err := EncryptData(compressedData, kv.encryptionKey)
		if err != nil {
			log.Println("saveToBytes: Error encrypting data:", err)
			return nil, fmt.Errorf("error encrypting data: %v", err)
		}
		log.Println("saveToBytes: Data encrypted successfully")
		return encryptedData, nil
	}
	log.Println("saveToBytes: Data saved without encryption")
	return compressedData, nil
}

// loadFromBytes loads the data from a byte slice into the in-memory data structure.
func (kv *KeyValueStore) loadFromBytes(data []byte) error {
	log.Println("loadFromBytes: Entered function")

	// Decode Base64 before decompress
	decodedData, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		log.Println("loadFromBytes: Error decoding Base64:", err)
		return fmt.Errorf("error decoding base64: %v", err)
	}

	log.Println("loadFromBytes: Decrypting data")
	decryptedData, err := DecryptData(decodedData, kv.encryptionKey)
	if err != nil {
		log.Println("loadFromBytes: Error decrypting data:", err)
		return fmt.Errorf("error decrypting data: %v", err)
	}

	log.Println("loadFromBytes: Decompressing data")
	decompressedData, err := DecompressData(decryptedData)
	if err != nil {
		log.Println("loadFromBytes: Error decompressing data:", err)
		return fmt.Errorf("error decompressing data: %v", err)
	}

	// Acquire the lock during unmarshalling
	kv.Lock()
	defer kv.Unlock()

	log.Println("loadFromBytes: Unmarshalling data")
	if err := json.Unmarshal(decompressedData, &kv.data); err != nil {
		log.Println("loadFromBytes: Error unmarshalling data:", err)
		return fmt.Errorf("error unmarshalling data: %v", err)
	}

	log.Println("loadFromBytes: Data loaded successfully")
	return nil
}
