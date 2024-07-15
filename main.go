package main

import (
	"bytes"
	"compress/zlib"
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

// KeyValueStore represents a simple key-value store with support for TTL, persistence, and encryption.
type KeyValueStore struct {
	sync.RWMutex
	data           map[string]interface{}
	expirations    map[string]time.Time
	filePath       string
	encryptionKey  []byte
	stopChan       chan struct{}
	cleanupStopped chan struct{}
	stopOnce       sync.Once
}

// NewKeyValueStore creates a new KeyValueStore instance and loads data from file if it exists.
func NewKeyValueStore(filePath string, encryptionKey []byte) *KeyValueStore {
	kv := &KeyValueStore{
		data:           make(map[string]interface{}),
		expirations:    make(map[string]time.Time),
		filePath:       filePath,
		encryptionKey:  encryptionKey,
		stopChan:       make(chan struct{}),
		cleanupStopped: make(chan struct{}),
	}

	if err := kv.load(); err != nil {
		log.Printf("Failed to load data: %v\n", err)
	}

	go kv.cleanupExpiredItems()

	return kv
}

// Stop stops the KeyValueStore instance and saves the data to the file.
func (kv *KeyValueStore) Stop() {
	kv.stopOnce.Do(func() {
		if kv.stopChan != nil {
			close(kv.stopChan)
			<-kv.cleanupStopped
		}
		if err := kv.save(); err != nil {
			log.Printf("Failed to save data: %v\n", err)
		}
	})
}

// Set sets a key-value pair in the store with an optional TTL.
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

// CompareAndSwap compares and swaps the value of a key if the current value matches the expected value.
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

// compressData compresses the given data using zlib.
func compressData(data []byte) ([]byte, error) {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	_, err := w.Write(data)
	if err != nil {
		return nil, err
	}
	err = w.Close()
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// decompressData decompresses the given data using zlib.
func decompressData(data []byte) ([]byte, error) {
	b := bytes.NewReader(data)
	r, err := zlib.NewReader(b)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
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

// save saves data to a file with compression and encryption.
func (kv *KeyValueStore) save() error {
	kv.RLock()
	defer kv.RUnlock()

	log.Println("Save: Acquired RLock")
	data, err := json.Marshal(kv.data)
	if err != nil {
		return fmt.Errorf("error marshalling data: %v", err)
	}

	compressedData, err := compressData(data)
	if err != nil {
		return fmt.Errorf("error compressing data: %v", err)
	}

	if len(kv.encryptionKey) > 0 {
		encryptedData, err := encryptData(compressedData, kv.encryptionKey)
		if err != nil {
			return fmt.Errorf("error encrypting data: %v", err)
		}
		data = []byte(encryptedData)
	} else {
		data = compressedData
	}

	if err := os.WriteFile(kv.filePath, data, 0644); err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}
	log.Println("Save: Released RLock")
	return nil
}

// load loads data from a file with decompression and decryption.
func (kv *KeyValueStore) load() error {
	kv.Lock()
	defer kv.Unlock()

	log.Println("Load: Acquired lock")
	file, err := os.Open(kv.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Load: No existing file, starting fresh")
			return nil
		}
		return fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	data, err := os.ReadFile(kv.filePath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	if len(kv.encryptionKey) > 0 {
		data, err = decryptData(string(data), kv.encryptionKey)
		if err != nil {
			return fmt.Errorf("error decrypting data: %v", err)
		}
	}

	decompressedData, err := decompressData(data)
	if err != nil {
		return fmt.Errorf("error decompressing data: %v", err)
	}

	if err := json.Unmarshal(decompressedData, &kv.data); err != nil {
		return fmt.Errorf("error unmarshalling data: %v", err)
	}
	log.Println("Load: Released lock")
	return nil
}

// cleanupExpiredItems runs periodically to remove expired items.
func (kv *KeyValueStore) cleanupExpiredItems() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			kv.Lock()
			now := time.Now()
			for key, exp := range kv.expirations {
				if now.After(exp) {
					delete(kv.data, key)
					delete(kv.expirations, key)
				}
			}
			kv.Unlock()
		case <-kv.stopChan:
			close(kv.cleanupStopped)
			return
		}
	}
}

// Example usage of KeyValueStore with persistence and encryption.
func main() {
	// Example of KeyValueStore use
	filePath := "data.json"
	encryptionKey := []byte("encryptionKey")

	// Initialize the KeyValueStore
	kv := NewKeyValueStore(filePath, encryptionKey)
	defer kv.Stop()

	// Set a key-value pair
	err := kv.Set("key1", "value1", 0)
	if err != nil {
		log.Fatalf("Error setting value: %v", err)
	}

	// Get the value for a key
	value, err := kv.Get("key1")
	if err != nil {
		log.Fatalf("Error getting value: %v", err)
	}
	log.Printf("Retrieved value: %v\n", value)

	// Simulate waiting for TTL expiration
	// In a real application, handle expiration asynchronously or with a ticker
	time.Sleep(5 * time.Second)
}
