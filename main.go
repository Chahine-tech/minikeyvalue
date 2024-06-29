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
	store           sync.Map
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
		persistenceFile: persistenceFile,
	}
	kv.loadFromDisk()
	go kv.cleanupExpiredItems()
	return kv
}

// Set sets a key-value pair in the store with an optional TTL
func (kv *KeyValueStore) Set(key, value string, ttl time.Duration) error {
	_, exists := kv.store.Load(key)
	if exists {
		return ErrKeyAlreadyExists
	}
	expiration := int64(0)
	if ttl > 0 {
		expiration = time.Now().Add(ttl).Unix()
	}
	kv.store.Store(key, Item{Value: value, Expiration: expiration})
	kv.saveToDisk()
	return nil
}

// Get retrieves a value by key from the store
func (kv *KeyValueStore) Get(key string) (string, error) {
	item, exists := kv.store.Load(key)
	if !exists {
		return "", ErrKeyNotExist
	}
	it := item.(Item)
	if it.Expiration > 0 && it.Expiration < time.Now().Unix() {
		return "", ErrKeyNotExist
	}
	return it.Value, nil
}

// Delete removes a key-value pair from the store
func (kv *KeyValueStore) Delete(key string) error {
	kv.store.Delete(key)
	kv.saveToDisk()
	fmt.Printf("Deleted key %s\n", key)
	return nil
}

// saveToDisk persists the store to disk
func (kv *KeyValueStore) saveToDisk() {
	data := make(map[string]Item)
	kv.store.Range(func(key, value interface{}) bool {
		data[key.(string)] = value.(Item)
		return true
	})
	bytes, err := json.Marshal(data)
	if err != nil {
		fmt.Println("Error saving to disk:", err)
		return
	}
	err = os.WriteFile(kv.persistenceFile, bytes, 0644)
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
	var items map[string]Item
	err = json.Unmarshal(data, &items)
	if err != nil {
		fmt.Println("Error loading from disk:", err)
		return
	}
	for key, item := range items {
		kv.store.Store(key, item)
	}
}

// cleanupExpiredItems periodically removes expired items from the store
func (kv *KeyValueStore) cleanupExpiredItems() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		expiredKeys := make([]string, 0)
		kv.store.Range(func(key, value interface{}) bool {
			item := value.(Item)
			if item.Expiration > 0 && item.Expiration < time.Now().Unix() {
				expiredKeys = append(expiredKeys, key.(string))
			}
			return true
		})
		for _, key := range expiredKeys {
			kv.store.Delete(key)
		}
		if len(expiredKeys) > 0 {
			kv.saveToDisk()
		}
	}
}

func main() {
	kv := NewKeyValueStore("kvstore.json")

	// Setting key-value pairs with and without TTL
	if err := kv.Set("name", "John", 0); err != nil {
		fmt.Println("Error setting key 'name':", err)
	}
	if err := kv.Set("session", "xyz123", 5*time.Second); err != nil {
		fmt.Println("Error setting key 'session':", err)
	}

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
	if err := kv.Delete("name"); err != nil {
		fmt.Println("Error deleting key 'name':", err)
	}

	// Trying to get a deleted key
	name, err = kv.Get("name")
	if err == nil {
		fmt.Println("Name:", name)
	} else {
		fmt.Println("Name key does not exist")
	}
}
