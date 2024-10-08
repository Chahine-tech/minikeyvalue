package store

import (
	"encoding/base64"
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
	globalTTL      time.Duration
	loaded         bool

	// Notification Manager
	notificationManager *NotificationManager
}

// NewKeyValueStore creates a new KeyValueStore instance without loading data initially.
func NewKeyValueStore(filePath string, encryptionKey []byte, globalTTL time.Duration, tickerInterval time.Duration) *KeyValueStore {
	kv := &KeyValueStore{
		data:                make(map[string][]KeyValue),
		expirations:         make(map[string]time.Time),
		filePath:            filePath,
		encryptionKey:       encryptionKey,
		stopChan:            make(chan struct{}),
		cleanupStopped:      make(chan struct{}),
		globalTTL:           globalTTL,
		notificationManager: NewNotificationManager(),
	}

	// Lazy loading: Data will be loaded only when needed
	log.Println("NewKeyValueStore: Instance created, lazy loading enabled.")

	go kv.cleanupExpiredItems(tickerInterval)

	return kv
}

func (kv *KeyValueStore) RegisterNotificationListener(listener func(string)) {
	kv.notificationManager.RegisterListener(listener)
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
	if err := kv.ensureLoaded(); err != nil {
		return err
	}

	now := time.Now()

	kv.RLock()
	_, exists := kv.data[key]
	kv.RUnlock()

	kv.Lock()
	defer kv.Unlock()

	if !exists {
		kv.data[key] = []KeyValue{}
	}

	kv.data[key] = append(kv.data[key], KeyValue{
		Value:     value,
		Timestamp: now,
	})

	if expiration > 0 {
		kv.expirations[key] = now.Add(expiration)
	} else if kv.globalTTL > 0 {
		kv.expirations[key] = now.Add(kv.globalTTL)
	} else {
		delete(kv.expirations, key)
	}

	if exists {
		kv.notificationManager.NotifyUpdate(key)
	} else {
		kv.notificationManager.NotifyAdd(key)
	}

	return nil
}

// Get retrieves the latest value for a given key from the store.
func (kv *KeyValueStore) Get(key string) (string, error) {
	log.Println("Get: Checking if data is loaded")
	if err := kv.ensureLoaded(); err != nil {
		return "", fmt.Errorf("data not loaded: %v", err)
	}

	kv.RLock()
	defer kv.RUnlock()

	values, exists := kv.data[key]
	if !exists || len(values) == 0 {
		return "", errors.New("key not found")
	}

	if exp, ok := kv.expirations[key]; ok && time.Now().After(exp) {
		return "", errors.New("key expired")
	}

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

// GetAllVersions retrieves all versions for a given key from the store.
func (kv *KeyValueStore) GetAllVersions(key string) ([]string, error) {
	kv.RLock()
	defer kv.RUnlock()

	if values, exists := kv.data[key]; exists {
		result := make([]string, len(values))
		for i, kv := range values {
			result[i] = kv.Value
		}
		return result, nil
	}
	return nil, errors.New("key not found")
}

// GetHistory retrieves the version history for a given key from the store.
func (kv *KeyValueStore) GetHistory(key string) ([]KeyValue, error) {
	kv.RLock()
	defer kv.RUnlock()

	if values, exists := kv.data[key]; exists {
		return values, nil
	}
	return nil, errors.New("key not found")
}

// RemoveVersion removes a specific version of a given key from the store.
func (kv *KeyValueStore) RemoveVersion(key string, version int) error {
	kv.Lock()
	defer kv.Unlock()

	versions, exists := kv.data[key]
	if !exists {
		return errors.New("key not found")
	}
	if version >= len(versions) {
		return errors.New("version not found")
	}

	kv.data[key] = append(versions[:version], versions[version+1:]...)
	return nil
}

// CompareAndSwap compares and swaps the value of a key if the current value matches the expected value.
func (kv *KeyValueStore) CompareAndSwap(key string, oldValue, newValue string, ttl time.Duration) (bool, error) {
	kv.Lock()
	defer kv.Unlock()

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
	return true, nil
}

// Delete removes a key from the store.
func (kv *KeyValueStore) Delete(key string) error {
	kv.Lock()
	defer kv.Unlock()

	if _, exists := kv.data[key]; !exists {
		return errors.New("key not found")
	}

	delete(kv.data, key)
	delete(kv.expirations, key)
	kv.notificationManager.NotifyDelete(key)

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

	compressedData, err := CompressData(data)
	if err != nil {
		return fmt.Errorf("error compressing data: %v", err)
	}

	var dataToWrite string
	if len(kv.encryptionKey) > 0 {
		log.Println("save: Encrypting data")
		encryptedData, err := EncryptData(compressedData, kv.encryptionKey)
		if err != nil {
			return fmt.Errorf("error encrypting data: %v", err)
		}
		// Base64 encode the encrypted data before writing to file
		dataToWrite = base64.StdEncoding.EncodeToString(encryptedData)
	} else {
		// Encode compressed data to Base64
		dataToWrite = base64.StdEncoding.EncodeToString(compressedData)
	}

	// Save the data (Base64 encoded)
	if err := os.WriteFile(kv.filePath, []byte(dataToWrite), 0644); err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}
	log.Println("Save: Released RLock")
	return nil
}

// load data from a file with decompression and decryption.
func (kv *KeyValueStore) load() error {
	log.Println("load: Starting to load data")

	file, err := os.Open(kv.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("load: No existing file, starting fresh")
			kv.loaded = true
			return nil
		}
		return fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	data, err := os.ReadFile(kv.filePath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	// Decode Base64
	decodedData, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return fmt.Errorf("error decoding base64: %v", err)
	}

	if len(kv.encryptionKey) > 0 {
		// Decrypt the data
		decodedData, err = DecryptData(decodedData, kv.encryptionKey)
		if err != nil {
			return fmt.Errorf("error decrypting data: %v", err)
		}
	}

	decompressedData, err := DecompressData(decodedData)
	if err != nil {
		return fmt.Errorf("error decompressing data: %v", err)
	}

	if err := json.Unmarshal(decompressedData, &kv.data); err != nil {
		return fmt.Errorf("error unmarshalling data: %v", err)
	}

	kv.loaded = true
	log.Println("load: Data loaded successfully")
	return nil
}

// Ensure data is loaded lazily
func (kv *KeyValueStore) ensureLoaded() error {
	kv.RLock()
	loaded := kv.loaded
	kv.RUnlock()
	if !loaded {
		kv.Lock()
		defer kv.Unlock()

		// Double-check to make sure another goroutine didn't load the data
		if !kv.loaded {
			log.Println("ensureLoaded: Triggering load")
			if err := kv.load(); err != nil {
				return fmt.Errorf("failed to load data: %v", err)
			}
			log.Println("ensureLoaded: Data loaded")
		}
	}
	return nil
}

func (kv *KeyValueStore) Loaded() bool {
	kv.RLock()
	defer kv.RUnlock()
	return kv.loaded
}
