package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// KeyValue represents a key-value pair with a timestamp.
type KeyValue struct {
	Value     string
	Timestamp time.Time
}

// KeyValueStore represents a simple key-value store with support for TTL, persistence, and encryption.
type KeyValueStore struct {
	sync.RWMutex
	data           map[string][]KeyValue
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
		data:           make(map[string][]KeyValue),
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
func (kv *KeyValueStore) Set(key, value string, expiration time.Duration) error {
	kv.Lock()
	defer kv.Unlock()

	log.Printf("Set: Acquired lock for key '%s'\n", key)
	now := time.Now()
	kv.data[key] = append(kv.data[key], KeyValue{
		Value:     value,
		Timestamp: now,
	})

	if expiration > 0 {
		kv.expirations[key] = now.Add(expiration)
	} else {
		delete(kv.expirations, key)
	}

	return nil
}

// Get retrieves the value for a given key from the store.
func (kv *KeyValueStore) Get(key string) (string, error) {
	kv.RLock()
	defer kv.RUnlock()

	log.Printf("Get: Acquired RLock for key '%s'\n", key)
	values, exists := kv.data[key]
	if !exists || len(values) == 0 {
		return "", errors.New("key not found")
	}
	if exp, ok := kv.expirations[key]; ok && time.Now().After(exp) {
		return "", errors.New("key expired")
	}
	log.Printf("Get: Released RLock for key '%s'\n", key)
	return values[len(values)-1].Value, nil
}

// GetVersion retrieves the value for the given key at the specified version
func (kv *KeyValueStore) GetVersion(key string, version int) (string, error) {
	kv.RLock()
	defer kv.RUnlock()

	versions, exists := kv.data[key]
	if !exists || version >= len(versions) {
		return "", errors.New("version not found")
	}

	return versions[version].Value, nil
}

// CompareAndSwap compares and swaps the value of a key if the current value matches the expected value.
func (kv *KeyValueStore) CompareAndSwap(key string, oldValue, newValue string, ttl time.Duration) (bool, error) {
	kv.Lock()
	defer kv.Unlock()

	log.Printf("CompareAndSwap: Acquired lock for key '%s'\n", key)
	values, exists := kv.data[key]
	if !exists || len(values) == 0 {
		log.Printf("CompareAndSwap: Key '%s' not found\n", key)
		return false, errors.New("key not found")
	}

	if values[len(values)-1].Value != oldValue {
		log.Printf("CompareAndSwap: Value mismatch for key '%s'. Expected: %v, Got: %v\n", key, oldValue, values[len(values)-1].Value)
		return false, nil
	}

	now := time.Now()
	kv.data[key] = append(kv.data[key], KeyValue{
		Value:     newValue,
		Timestamp: now,
	})
	if ttl > 0 {
		kv.expirations[key] = now.Add(ttl)
	} else {
		delete(kv.expirations, key)
	}
	log.Printf("CompareAndSwap: Released lock for key '%s'\n", key)
	return true, nil
}

// Delete removes a key from the store.
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

// load data from a file with decompression and decryption.
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
