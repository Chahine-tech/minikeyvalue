package main

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
	"os"
	"sync"
	"time"
)

// KeyValueStore represents a simple key-value store that supports setting and getting values.
type KeyValueStore struct {
	sync.RWMutex                          // Mutex for synchronizing concurrent access
	data           map[string]interface{} // Stores key-value data
	expirations    map[string]time.Time   // Tracks key expirations
	filePath       string                 // File path for backup
	encryptionKey  []byte                 // Encryption key for data
	stopChan       chan struct{}          // Channel to stop expired items cleanup
	cleanupStopped chan struct{}          // Channel to signal cleanup stop
}

// NewKeyValueStore creates a new instance of KeyValueStore.
func NewKeyValueStore(filePath string, encryptionKey []byte) *KeyValueStore {
	kv := &KeyValueStore{
		data:           make(map[string]interface{}),
		expirations:    make(map[string]time.Time),
		filePath:       filePath,
		encryptionKey:  encryptionKey,
		stopChan:       make(chan struct{}),
		cleanupStopped: make(chan struct{}),
	}

	// Load data from file if it exists
	if err := kv.load(); err != nil {
		log.Printf("Failed to load data: %v\n", err)
	}

	// Start periodic cleanup of expired items
	go kv.cleanupExpiredItems()

	return kv
}

// Stop stops the KeyValueStore instance.
func (kv *KeyValueStore) Stop() {
	close(kv.stopChan)  // Signal to stop periodic cleanup
	<-kv.cleanupStopped // Wait for cleanup to finish
	if err := kv.save(); err != nil {
		log.Printf("Failed to save data: %v\n", err)
	}
}

// Set sets a key-value pair in the store.
func (kv *KeyValueStore) Set(key string, value interface{}, ttl time.Duration) error {
	kv.Lock()
	defer kv.Unlock()

	log.Printf("Set: Acquired lock for key '%s'\n", key)
	kv.data[key] = value
	if ttl > 0 {
		kv.expirations[key] = time.Now().Add(ttl)
	} else {
		delete(kv.expirations, key)
	}
	log.Printf("Set: Released lock for key '%s'\n", key)
	return nil
}

// Get retrieves the value for a given key from the store.
func (kv *KeyValueStore) Get(key string) (interface{}, error) {
	kv.RLock()
	defer kv.RUnlock()

	log.Printf("Get: Acquired RLock for key '%s'\n", key)
	value, exists := kv.data[key]
	if !exists {
		return nil, errors.New("key not found")
	}
	if exp, ok := kv.expirations[key]; ok && time.Now().After(exp) {
		return nil, errors.New("key expired")
	}
	log.Printf("Get: Released RLock for key '%s'\n", key)
	return value, nil
}

// CompareAndSwap compares the value of a key with an expected value and swaps it with a new value if the comparison succeeds.
func (kv *KeyValueStore) CompareAndSwap(key string, oldValue, newValue interface{}, ttl time.Duration) (bool, error) {
	kv.Lock()
	defer kv.Unlock()

	log.Printf("CompareAndSwap: Acquired lock for key '%s'\n", key)
	value, exists := kv.data[key]
	if !exists {
		log.Printf("CompareAndSwap: Key '%s' not found\n", key)
		return false, errors.New("key not found")
	}

	if value != oldValue {
		log.Printf("CompareAndSwap: Value mismatch for key '%s'. Expected: %v, Got: %v\n", key, oldValue, value)
		return false, nil
	}

	kv.data[key] = newValue
	if ttl > 0 {
		kv.expirations[key] = time.Now().Add(ttl)
	} else {
		delete(kv.expirations, key)
	}
	log.Printf("CompareAndSwap: Released lock for key '%s'\n", key)
	return true, nil
}

// Delete deletes a key from the store.
func (kv *KeyValueStore) Delete(key string) error {
	kv.Lock()
	defer kv.Unlock()

	log.Printf("Delete: Acquired lock for key '%s'\n", key)
	delete(kv.data, key)
	delete(kv.expirations, key)
	log.Printf("Delete: Released lock for key '%s'\n", key)
	return nil
}

// Keys returns a list of all keys in the store.
func (kv *KeyValueStore) Keys() []string {
	kv.RLock()
	defer kv.RUnlock()

	log.Println("Keys: Acquired RLock")
	keys := make([]string, 0, len(kv.data))
	for key := range kv.data {
		keys = append(keys, key)
	}
	log.Println("Keys: Released RLock")
	return keys
}

// Size returns the number of key-value pairs in the store.
func (kv *KeyValueStore) Size() int {
	kv.RLock()
	defer kv.RUnlock()

	log.Println("Size: Acquired RLock")
	size := len(kv.data)
	log.Println("Size: Released RLock")
	return size
}

// encryptData encrypts the given data using the provided encryption key.
func encryptData(data []byte, key []byte) (string, error) {
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
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptData decrypts the given data using the provided encryption key.
func decryptData(encryptedData string, key []byte) ([]byte, error) {
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
		return nil, errors.New("malformed ciphertext")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// save saves data to a file.
func (kv *KeyValueStore) save() error {
	kv.RLock()
	defer kv.RUnlock()

	log.Println("Save: Acquired RLock")
	data, err := json.Marshal(kv.data)
	if err != nil {
		return fmt.Errorf("error marshalling data: %v", err)
	}

	// Encrypt data here using kv.encryptionKey (if provided)
	if len(kv.encryptionKey) > 0 {
		encryptedData, err := encryptData(data, kv.encryptionKey)
		if err != nil {
			return fmt.Errorf("error encrypting data: %v", err)
		}
		data = []byte(encryptedData)
	}

	if err := os.WriteFile(kv.filePath, data, 0644); err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}
	log.Println("Save: Released RLock")
	return nil
}

// load loads data from a file.
func (kv *KeyValueStore) load() error {
	kv.Lock()
	defer kv.Unlock()

	log.Println("Load: Acquired lock")
	file, err := os.Open(kv.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Load: No existing file, starting fresh")
			return nil // No existing file is fine, start fresh
		}
		return fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	data, err := os.ReadFile(kv.filePath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	// Decrypt data here using kv.encryptionKey (if provided)
	if len(kv.encryptionKey) > 0 {
		decryptedData, err := decryptData(string(data), kv.encryptionKey)
		if err != nil {
			return fmt.Errorf("error decrypting data: %v", err)
		}
		data = decryptedData
	}

	if err := json.Unmarshal(data, &kv.data); err != nil {
		return fmt.Errorf("error unmarshalling data: %v", err)
	}
	log.Println("Load: Released lock")
	return nil
}

// cleanupExpiredItems periodically checks for expired items and deletes them.
func (kv *KeyValueStore) cleanupExpiredItems() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			kv.Lock()
			log.Println("Cleanup: Acquired lock")
			now := time.Now()
			for key, exp := range kv.expirations {
				if now.After(exp) {
					delete(kv.data, key)
					delete(kv.expirations, key)
					log.Printf("Cleanup: Expired key '%s' removed\n", key)
				}
			}
			log.Println("Cleanup: Released lock")
			kv.Unlock()
		case <-kv.stopChan:
			kv.cleanupStopped <- struct{}{}
			return
		}
	}
}

func main() {
	// Example usage:
	filePath := "data.json"
	encryptionKey := []byte("exampleEncryptionKey") // Must be 16, 24 or 32 bytes for AES-128, AES-192, or AES-256 respectively
	kv := NewKeyValueStore(filePath, encryptionKey)
	defer kv.Stop()

	// Test operations
	err := kv.Set("key1", "value1", 5*time.Second)
	if err != nil {
		log.Printf("Failed to set key1: %v\n", err)
	}

	value, err := kv.Get("key1")
	if err != nil {
		log.Printf("Error getting key1: %v\n", err)
	} else {
		log.Printf("Got key1 value: %v\n", value)
	}

	time.Sleep(6 * time.Second) // Wait for key1 to expire
	value, err = kv.Get("key1")
	if err != nil {
		log.Printf("Error getting expired key1: %v\n", err)
	} else {
		log.Printf("Got expired key1 value: %v\n", value)
	}
}
