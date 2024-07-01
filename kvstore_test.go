package main

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func TestKeyValueStore(t *testing.T) {
	const persistenceFile = "test_kvstore.json"
	os.Remove(persistenceFile)
	defer os.Remove(persistenceFile)

	kv := NewKeyValueStore(persistenceFile)
	defer kv.Stop()

	// Test setting and getting a key
	if err := kv.Set("name", "John", 0); err != nil {
		t.Errorf("error setting key 'name': %v", err)
	}
	value, err := kv.Get("name")
	if err != nil || value != "John" {
		t.Errorf("expected value 'John', got '%v' (error: %v)", value, err)
	}

	// Test setting a key with TTL
	if err := kv.Set("session", "xyz123", 2*time.Second); err != nil {
		t.Errorf("error setting key 'session': %v", err)
	}
	value, err = kv.Get("session")
	if err != nil || value != "xyz123" {
		t.Errorf("expected value 'xyz123', got '%v' (error: %v)", value, err)
	}

	// Wait for the TTL to expire
	time.Sleep(3 * time.Second)
	if _, err = kv.Get("session"); err == nil {
		t.Errorf("expected 'session' key to have expired")
	}

	// Test deleting a key
	if err := kv.Delete("name"); err != nil {
		t.Errorf("error deleting key 'name': %v", err)
	}
	if _, err = kv.Get("name"); err == nil {
		t.Errorf("expected 'name' key to be deleted")
	}

	// Test persistence
	if err := kv.Set("name", "Jane", 0); err != nil {
		t.Errorf("error setting key 'name': %v", err)
	}
	kv2 := NewKeyValueStore(persistenceFile)
	defer kv2.Stop()
	value, err = kv2.Get("name")
	if err != nil || value != "Jane" {
		t.Errorf("expected value 'Jane' after loading from disk, got '%v' (error: %v)", value, err)
	}

	// Test Keys() method
	keys := kv2.Keys()
	if len(keys) != 1 || keys[0] != "name" {
		t.Errorf("expected Keys() to return ['name'], got %v", keys)
	}

	// Test Size() method
	size := kv2.Size()
	if size != 1 {
		t.Errorf("expected Size() to return 1, got %d", size)
	}

	// Test atomic update
	if err := kv.Set("counter", 10, 0); err != nil {
		t.Errorf("error setting key 'counter': %v", err)
	}
	updateFunc := func(value interface{}) interface{} {
		return value.(int) + 1
	}
	if err := kv.Update("counter", updateFunc); err != nil {
		t.Errorf("error updating key 'counter': %v", err)
	}
	updatedValue, err := kv.Get("counter")
	if err != nil || updatedValue != 11 {
		t.Errorf("expected value '11' after update, got '%v' (error: %v)", updatedValue, err)
	}
}

func TestCleanupExpiredItems(t *testing.T) {
	fmt.Println("Starting TestCleanupExpiredItems")
	const persistenceFile = "test_kvstore_cleanup.json"
	os.Remove(persistenceFile)
	defer os.Remove(persistenceFile)

	kv := NewKeyValueStore(persistenceFile)
	defer kv.Stop()

	if err := kv.Set("temp", "data", 1*time.Second); err != nil {
		t.Errorf("error setting key 'temp': %v", err)
	}
	time.Sleep(3 * time.Second) // Extended sleep to ensure cleanup runs

	if _, err := kv.Get("temp"); err == nil {
		t.Errorf("expected 'temp' key to have expired")
	}

	// Waiting to observe the cleanup cycle in action
	time.Sleep(2 * time.Second)
	fmt.Println("Finished TestCleanupExpiredItems")
}

func TestKeyValueStoreConcurrency(t *testing.T) {
	fmt.Println("Starting TestKeyValueStoreConcurrency")
	const persistenceFile = "test_kvstore_concurrency.json"
	os.Remove(persistenceFile)
	defer os.Remove(persistenceFile)

	kv := NewKeyValueStore(persistenceFile)
	defer kv.Stop()

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := kv.Set(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i), 0); err != nil {
				t.Errorf("error setting key 'key%d': %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			value, err := kv.Get(fmt.Sprintf("key%d", i))
			if err != nil {
				t.Errorf("error getting key 'key%d': %v", i, err)
			} else if value != fmt.Sprintf("value%d", i) {
				t.Errorf("expected value 'value%d', got '%v'", i, value)
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < 100; i++ {
		value, err := kv.Get(fmt.Sprintf("key%d", i))
		if err != nil || value != fmt.Sprintf("value%d", i) {
			t.Errorf("expected value 'value%d', got '%v' (error: %v)", i, value, err)
		}
	}

	// Test Keys() and Size() methods after concurrent operations
	keys := kv.Keys()
	if len(keys) != 100 {
		t.Errorf("expected 100 keys, got %d", len(keys))
	}

	size := kv.Size()
	if size != 100 {
		t.Errorf("expected Size() to return 100, got %d", size)
	}
}
