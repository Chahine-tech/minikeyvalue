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

type KeyValueStore struct {
	mutex           sync.RWMutex
	store           map[string]Item
	persistenceFile string
	stopChan        chan struct{}
}

type Item struct {
	Value      interface{}
	Expiration int64
}

func NewKeyValueStore(persistenceFile string) *KeyValueStore {
	kv := &KeyValueStore{
		store:           make(map[string]Item),
		persistenceFile: persistenceFile,
		stopChan:        make(chan struct{}),
	}
	kv.loadFromDisk()
	go kv.cleanupExpiredItems()
	return kv
}

func (kv *KeyValueStore) Set(key string, value interface{}, ttl time.Duration) error {
	kv.mutex.Lock()
	defer kv.mutex.Unlock()

	expiration := int64(0)
	if ttl > 0 {
		expiration = time.Now().Add(ttl).Unix()
	}
	kv.store[key] = Item{Value: value, Expiration: expiration}
	kv.saveToDisk()
	return nil
}

func (kv *KeyValueStore) Get(key string) (interface{}, error) {
	kv.mutex.RLock()
	defer kv.mutex.RUnlock()

	item, exists := kv.store[key]
	if !exists {
		return nil, ErrKeyNotExist
	}
	if item.Expiration > 0 && item.Expiration < time.Now().Unix() {
		delete(kv.store, key)
		kv.saveToDisk()
		return nil, ErrKeyNotExist
	}
	return item.Value, nil
}

func (kv *KeyValueStore) Delete(key string) error {
	kv.mutex.Lock()
	defer kv.mutex.Unlock()

	_, exists := kv.store[key]
	if !exists {
		return ErrKeyNotExist
	}

	delete(kv.store, key)
	kv.saveToDisk()
	fmt.Printf("Deleted key %s\n", key)
	return nil
}

// Update atomically updates the value for a given key using the provided update function.
func (kv *KeyValueStore) Update(key string, updateFunc func(interface{}) interface{}) error {
	kv.mutex.Lock()
	defer kv.mutex.Unlock()

	item, exists := kv.store[key]
	if !exists {
		return ErrKeyNotExist
	}

	// Apply the update function to the current value
	item.Value = updateFunc(item.Value)
	kv.store[key] = item
	kv.saveToDisk()
	return nil
}

func (kv *KeyValueStore) saveToDisk() {
	bytes, err := json.Marshal(kv.store)
	if err != nil {
		fmt.Println("Error saving to disk:", err)
		return
	}
	err = os.WriteFile(kv.persistenceFile, bytes, 0644)
	if err != nil {
		fmt.Println("Error saving to disk:", err)
	}
}

func (kv *KeyValueStore) loadFromDisk() {
	data, err := os.ReadFile(kv.persistenceFile)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Println("Error loading from disk:", err)
		}
		return
	}
	err = json.Unmarshal(data, &kv.store)
	if err != nil {
		fmt.Println("Error loading from disk:", err)
	}
}

func (kv *KeyValueStore) cleanupExpiredItems() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			kv.mutex.Lock()
			now := time.Now().Unix()
			for key, item := range kv.store {
				if item.Expiration > 0 && item.Expiration < now {
					delete(kv.store, key)
				}
			}
			kv.saveToDisk()
			kv.mutex.Unlock()
		case <-kv.stopChan:
			return
		}
	}
}

func (kv *KeyValueStore) Stop() {
	close(kv.stopChan)
}

func (kv *KeyValueStore) Keys() []string {
	kv.mutex.RLock()
	defer kv.mutex.RUnlock()

	keys := make([]string, 0, len(kv.store))
	for k := range kv.store {
		keys = append(keys, k)
	}
	return keys
}

func (kv *KeyValueStore) Size() int {
	kv.mutex.RLock()
	defer kv.mutex.RUnlock()
	return len(kv.store)
}

func main() {
	kv := NewKeyValueStore("kvstore.json")
	defer kv.Stop()

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

	// Atomically update a key
	updateErr := kv.Update("name", func(value interface{}) interface{} {
		return value.(string) + " Doe"
	})
	if updateErr != nil {
		fmt.Println("Error updating key 'name':", updateErr)
	}

	// Getting the updated value
	updatedName, err := kv.Get("name")
	if err == nil {
		fmt.Println("Updated Name:", updatedName)
	} else {
		fmt.Println("Name key does not exist")
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
