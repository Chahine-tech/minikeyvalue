package main

import (
	"os"
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
	kv.Set("name", "John", 0)
	if value, exists := kv.Get("name"); !exists || value != "John" {
		t.Errorf("expected value 'John', got '%s'", value)
	}

	// Test setting a key with TTL
	kv.Set("session", "xyz123", 2*time.Second)
	if value, exists := kv.Get("session"); !exists || value != "xyz123" {
		t.Errorf("expected value 'xyz123', got '%s'", value)
	}

	// Wait for the TTL to expire
	time.Sleep(3 * time.Second)
	if _, exists := kv.Get("session"); exists {
		t.Errorf("expected 'session' key to have expired")
	}

	// Test deleting a key
	kv.Delete("name")
	if _, exists := kv.Get("name"); exists {
		t.Errorf("expected 'name' key to be deleted")
	}

	// Test persistence
	kv.Set("name", "Jane", 0)
	kv2 := NewKeyValueStore(persistenceFile)
	if value, exists := kv2.Get("name"); !exists || value != "Jane" {
		t.Errorf("expected value 'Jane' after loading from disk, got '%s'", value)
	}
}

func TestCleanupExpiredItems(t *testing.T) {
	const persistenceFile = "test_kvstore_cleanup.json"
	os.Remove(persistenceFile)
	defer os.Remove(persistenceFile)

	kv := NewKeyValueStore(persistenceFile)
	kv.Set("temp", "data", 1*time.Second)
	time.Sleep(2 * time.Second)

	if _, exists := kv.Get("temp"); exists {
		t.Errorf("expected 'temp' key to have expired")
	}
}
