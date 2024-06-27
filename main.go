package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	ErrKeyNotExist      = errors.New("key does not exist")
	ErrKeyAlreadyExists = errors.New("key already exists")
)

// KeyValueStore is an advanced thread-safe in-memory key-value store with persistence and TTL
type KeyValueStore struct {
	store           map[string]Item
	mu              sync.RWMutex
	persistenceFile string
}

// Item represents a value with an optional expiration time
type Item struct {
	Value      string
	Expiration int64
}

// NewKeyValueStore creates a new KeyValueStore with persistence
func NewKeyValueStore(persistenceFile string) *KeyValueStore {
	kv := &KeyValueStore{
		store:           make(map[string]Item),
		persistenceFile: persistenceFile,
	}
	kv.loadFromDisk()
	go kv.cleanupExpiredItems()
	return kv
}

// Set sets a key-value pair in the store with an optional TTL
func (kv *KeyValueStore) Set(key, value string, ttl time.Duration) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if _, exists := kv.store[key]; exists {
		return ErrKeyAlreadyExists
	}
	expiration := int64(0)
	if ttl > 0 {
		expiration = time.Now().Add(ttl).Unix()
	}
	kv.store[key] = Item{Value: value, Expiration: expiration}
	kv.saveToDisk()
	return nil
}

// Get retrieves a value by key from the store
func (kv *KeyValueStore) Get(key string) (string, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	item, exists := kv.store[key]
	if !exists || (item.Expiration > 0 && item.Expiration < time.Now().Unix()) {
		return "", ErrKeyNotExist
	}
	return item.Value, nil
}

// Delete removes a key-value pair from the store
func (kv *KeyValueStore) Delete(key string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	delete(kv.store, key)
	kv.saveToDisk()
	fmt.Printf("Deleted key %s\n", key)
}

// saveToDisk persists the store to disk
func (kv *KeyValueStore) saveToDisk() {
	data, err := json.Marshal(kv.store)
	if err != nil {
		fmt.Println("Error saving to disk:", err)
		return
	}
	err = os.WriteFile(kv.persistenceFile, data, 0644)
	if err != nil {
		fmt.Println("Error saving to disk:", err)
	}
}

// loadFromDisk loads the store from disk
func (kv *KeyValueStore) loadFromDisk() {
	_, err := os.Stat(kv.persistenceFile)
	if os.IsNotExist(err) {
		// File does not exist, create an empty file
		emptyFile, err := os.Create(kv.persistenceFile)
		if err != nil {
			fmt.Println("Error creating persistence file:", err)
			return
		}
		emptyFile.Close()
		return
	}

	data, err := os.ReadFile(kv.persistenceFile)
	if err != nil {
		fmt.Println("Error loading from disk:", err)
		return
	}
	err = json.Unmarshal(data, &kv.store)
	if err != nil {
		fmt.Println("Error loading from disk:", err)
	}
}

// cleanupExpiredItems periodically removes expired items from the store
func (kv *KeyValueStore) cleanupExpiredItems() {
	for {
		time.Sleep(1 * time.Minute)
		kv.mu.Lock()
		now := time.Now().Unix()
		for key, item := range kv.store {
			if item.Expiration > 0 && item.Expiration < now {
				delete(kv.store, key)
			}
		}
		kv.mu.Unlock()
	}
}

func main() {
	kv := NewKeyValueStore("kvstore.json")

	// Setting key-value pairs with and without TTL
	kv.Set("name", "John", 0)
	kv.Set("session", "xyz123", 5*time.Second)

	// Getting a value
	name, err := kv.Get("name")
	if err == nil {
		fmt.Println("Name:", name)
	} else {
		fmt.Println("Name key does not exist")
	}

	// Trying to get a key with TTL
	fmt.Println("Waiting for 6 seconds...")
	time.Sleep(6 * time.Second)
	session, err := kv.Get("session")
	if err == nil {
		fmt.Println("Session:", session)
	} else {
		fmt.Println("Session key has expired or does not exist")
	}

	// Deleting a key
	kv.Delete("name")

	// Trying to get a deleted key
	name, err = kv.Get("name")
	if err == nil {
		fmt.Println("Name:", name)
	} else {
		fmt.Println("Name key does not exist")
	}
}
