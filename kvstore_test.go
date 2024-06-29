package main

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func TestKeyValueStore(t *testing.T) {
	// Clean up persistence file before and after the test
	const persistenceFile = "test_kvstore.json"
	os.Remove(persistenceFile)
	defer os.Remove(persistenceFile)

	kv := NewKeyValueStore(persistenceFile)

	// Test setting and getting a key
	if err := kv.Set("name", "John", 0); err != nil {
		t.Errorf("error setting key 'name': %v", err)
	}
	value, err := kv.Get("name")
	if err != nil || value != "John" {
		t.Errorf("expected value 'John', got '%s' (error: %v)", value, err)
	}

	// Test setting a key with TTL
	if err := kv.Set("session", "xyz123", 2*time.Second); err != nil {
		t.Errorf("error setting key 'session': %v", err)
	}
	value, err = kv.Get("session")
	if err != nil || value != "xyz123" {
		t.Errorf("expected value 'xyz123', got '%s' (error: %v)", value, err)
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
	value, err = kv2.Get("name")
	if err != nil || value != "Jane" {
		t.Errorf("expected value 'Jane' after loading from disk, got '%s' (error: %v)", value, err)
	}
}

func TestCleanupExpiredItems(t *testing.T) {
	const persistenceFile = "test_kvstore_cleanup.json"
	os.Remove(persistenceFile)
	defer os.Remove(persistenceFile)

	kv := NewKeyValueStore(persistenceFile)
	if err := kv.Set("temp", "data", 1*time.Second); err != nil {
		t.Errorf("error setting key 'temp': %v", err)
	}
	time.Sleep(2 * time.Second)

	if _, err := kv.Get("temp"); err == nil {
		t.Errorf("expected 'temp' key to have expired")
	}
}

func TestKeyValueStoreConcurrency(t *testing.T) {
	const persistenceFile = "test_kvstore_concurrency.json"
	os.Remove(persistenceFile)
	defer os.Remove(persistenceFile)

	kv := NewKeyValueStore(persistenceFile)
	var wg sync.WaitGroup

	// Test concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := kv.Set(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i), 0); err != nil {
				t.Errorf("error setting key 'key%d': %v", i, err)
			}
		}(i)
	}

	// Wait for all writes to complete
	wg.Wait()

	// Adding a short delay to ensure all writes are complete
	time.Sleep(100 * time.Millisecond)

	// Test concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			value, err := kv.Get(fmt.Sprintf("key%d", i))
			if err != nil {
				t.Errorf("error getting key 'key%d': %v", i, err)
			} else if value != fmt.Sprintf("value%d", i) {
				t.Errorf("expected value 'value%d', got '%s'", i, value)
			}
		}(i)
	}

	// Wait for all reads to complete
	wg.Wait()

	// Verify all keys
	for i := 0; i < 100; i++ {
		value, err := kv.Get(fmt.Sprintf("key%d", i))
		if err != nil || value != fmt.Sprintf("value%d", i) {
			t.Errorf("expected value 'value%d', got '%s' (error: %v)", i, value, err)
		}
	}
}
