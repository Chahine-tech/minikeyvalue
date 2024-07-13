package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sync"
	"testing"
	"time"
)

var encryptionKey = []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

func init() {
	go func() {
		fmt.Println("Starting pprof on http://localhost:6060")
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			log.Fatalf("Failed to start pprof server: %v", err)
		}
	}()
}

func TestKeyValueStore(t *testing.T) {
	filePath := "test_store.json"
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

	// Initialize KeyValueStore
	kvStore := NewKeyValueStore(filePath, encryptionKey)

	// Ensure we clean up and persist data
	defer func() {
		kvStore.Stop()
		os.Remove(filePath)
	}()

	// Set key 'name' with value 'Jane'
	err := kvStore.Set("name", "Jane", 0)
	if err != nil {
		t.Fatalf("Failed to set key 'name': %v", err)
	}

	// Test Get operation for key 'name'
	value, err := kvStore.Get("name")
	if err != nil {
		t.Fatalf("Failed to get key 'name': %v", err)
	}

	// Assert that the retrieved value matches the expected value 'Jane'
	expected := "Jane"
	if value != expected {
		t.Errorf("Expected value '%s', got '%v'", expected, value)
	}

	// Restart the KeyValueStore to ensure data is persisted correctly
	kvStore.Stop()
	kvStore = NewKeyValueStore(filePath, encryptionKey)

	// Test Get operation again for key 'name' after restart
	value, err = kvStore.Get("name")
	if err != nil {
		t.Fatalf("Failed to get key 'name' after restart: %v", err)
	}
	if value != expected {
		t.Errorf("Expected value '%s' after restart, got '%v'", expected, value)
	}

	// Clean up after test
	kvStore.Stop()
	os.Remove(filePath)
}

func TestCleanupExpiredItems(t *testing.T) {
	fmt.Println("Running TestCleanupExpiredItems")
	const persistenceFile = "test_kvstore_cleanup.json"
	os.Remove(persistenceFile)
	defer os.Remove(persistenceFile)

	kv := NewKeyValueStore(persistenceFile, encryptionKey)
	defer kv.Stop()

	if err := kv.Set("temp", "data", 1*time.Second); err != nil {
		t.Errorf("error setting key 'temp': %v", err)
	}
	time.Sleep(3 * time.Second)

	if _, err := kv.Get("temp"); err == nil {
		t.Errorf("expected 'temp' key to have expired")
	}

	fmt.Println("Finished TestCleanupExpiredItems")
}

func TestKeyValueStoreConcurrency(t *testing.T) {
	fmt.Println("Running TestKeyValueStoreConcurrency")
	const persistenceFile = "test_kvstore_concurrency.json"
	os.Remove(persistenceFile)
	defer os.Remove(persistenceFile)

	kv := NewKeyValueStore(persistenceFile, encryptionKey)
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

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			value, err := kv.Get(fmt.Sprintf("key%d", i))
			if err != nil || value != fmt.Sprintf("value%d", i) {
				t.Errorf("expected value 'value%d', got '%v' (error: %v)", i, value, err)
			}
		}(i)
	}
	wg.Wait()

	fmt.Println("TestKeyValueStoreConcurrency completed")
}

func TestCompressionDecompression(t *testing.T) {
	fmt.Println("Running TestCompressionDecompression")

	originalData := "This is a test string to be compressed and decompressed"
	compressedData, err := compressData([]byte(originalData))
	if err != nil {
		t.Fatalf("Failed to compress data: %v", err)
	}

	decompressedData, err := decompressData(compressedData)
	if err != nil {
		t.Fatalf("Failed to decompress data: %v", err)
	}

	if string(decompressedData) != originalData {
		t.Errorf("Expected decompressed data to be '%s', got '%s'", originalData, decompressedData)
	}

	fmt.Println("Finished TestCompressionDecompression")
}

func TestSetWithTTL(t *testing.T) {
	filePath := "test_store_with_ttl.json"
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

	kvStore := NewKeyValueStore(filePath, encryptionKey)
	defer func() {
		kvStore.Stop()
		os.Remove(filePath)
	}()

	err := kvStore.Set("temp", "value", 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to set key 'temp': %v", err)
	}

	time.Sleep(3 * time.Second)

	_, err = kvStore.Get("temp")
	if err == nil {
		t.Errorf("Expected key 'temp' to have expired")
	}
}

func TestGetNonExistentKey(t *testing.T) {
	filePath := "test_store_nonexistent_key.json"
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

	kvStore := NewKeyValueStore(filePath, encryptionKey)
	defer func() {
		kvStore.Stop()
		os.Remove(filePath)
	}()

	_, err := kvStore.Get("nonexistent")
	if err == nil {
		t.Errorf("Expected error when getting non-existent key")
	}
}

func TestCompareAndSwap(t *testing.T) {
	filePath := "test_store_compare_and_swap.json"
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

	kvStore := NewKeyValueStore(filePath, encryptionKey)
	defer func() {
		kvStore.Stop()
		os.Remove(filePath)
	}()

	err := kvStore.Set("key1", "value1", 0)
	if err != nil {
		t.Fatalf("Failed to set key 'key1': %v", err)
	}

	swapped, err := kvStore.CompareAndSwap("key1", "value1", "value2", 0)
	if err != nil || !swapped {
		t.Fatalf("Failed to swap value for key 'key1'")
	}

	value, err := kvStore.Get("key1")
	if err != nil || value != "value2" {
		t.Errorf("Expected value 'value2', got '%v'", value)
	}

	swapped, err = kvStore.CompareAndSwap("key1", "value1", "value3", 0)
	if err != nil || swapped {
		t.Errorf("Expected CompareAndSwap to fail with old value 'value1'")
	}
}

func TestDelete(t *testing.T) {
	filePath := "test_store_delete.json"
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

	kvStore := NewKeyValueStore(filePath, encryptionKey)
	defer func() {
		kvStore.Stop()
		os.Remove(filePath)
	}()

	err := kvStore.Set("key1", "value1", 0)
	if err != nil {
		t.Fatalf("Failed to set key 'key1': %v", err)
	}

	err = kvStore.Delete("key1")
	if err != nil {
		t.Fatalf("Failed to delete key 'key1': %v", err)
	}

	_, err = kvStore.Get("key1")
	if err == nil {
		t.Errorf("Expected error when getting deleted key 'key1'")
	}
}

func TestKeysAfterDeletionAndExpiration(t *testing.T) {
	filePath := "test_store_keys_after_delete.json"
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

	kvStore := NewKeyValueStore(filePath, encryptionKey)
	defer func() {
		kvStore.Stop()
		os.Remove(filePath)
	}()

	err := kvStore.Set("key1", "value1", 1*time.Second)
	if err != nil {
		t.Fatalf("Failed to set key 'key1': %v", err)
	}

	err = kvStore.Set("key2", "value2", 0)
	if err != nil {
		t.Fatalf("Failed to set key 'key2': %v", err)
	}

	time.Sleep(2 * time.Second) // Wait for key1 to expire

	keys := kvStore.Keys()
	expectedKeys := []string{"key2"}
	if len(keys) != len(expectedKeys) {
		t.Errorf("Expected keys %v, got %v", expectedKeys, keys)
	}
}

func TestSize(t *testing.T) {
	filePath := "test_store_size.json"
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

	kvStore := NewKeyValueStore(filePath, encryptionKey)
	defer func() {
		kvStore.Stop()
		os.Remove(filePath)
	}()

	err := kvStore.Set("key1", "value1", 0)
	if err != nil {
		t.Fatalf("Failed to set key 'key1': %v", err)
	}

	err = kvStore.Set("key2", "value2", 0)
	if err != nil {
		t.Fatalf("Failed to set key 'key2': %v", err)
	}

	size := kvStore.Size()
	expectedSize := 2
	if size != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, size)
	}
}

func TestCompareAndSwapConcurrency(t *testing.T) {
	filePath := "test_store_cas_concurrency.json"
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

	kvStore := NewKeyValueStore(filePath, encryptionKey)
	defer func() {
		kvStore.Stop()
		os.Remove(filePath)
	}()

	err := kvStore.Set("key1", "value1", 0)
	if err != nil {
		t.Fatalf("Failed to set key 'key1': %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			kvStore.CompareAndSwap("key1", "value1", fmt.Sprintf("value%d", i), 0)
		}(i)
	}
	wg.Wait()

	value, err := kvStore.Get("key1")
	if err != nil {
		t.Fatalf("Failed to get key 'key1': %v", err)
	}
	if value == "value1" {
		t.Errorf("Expected value 'key1' to be changed from 'value1', but it was not")
	}
}
