package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Chahine-tech/minikeyvalue/internal/store"
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
	encryptionKey := []byte("0123456789abcdef") // 16 bytes key for AES-128

	// Initialize KeyValueStore
	kvStore := store.NewKeyValueStore(filePath, encryptionKey)

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
	kvStore = store.NewKeyValueStore(filePath, encryptionKey)

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

	kv := store.NewKeyValueStore(persistenceFile, encryptionKey)
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

	kv := store.NewKeyValueStore(persistenceFile, encryptionKey)
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

func TestCompareAndSwapConcurrency(t *testing.T) {
	filePath := "test_store_cas_concurrency.json"
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

	kvStore := store.NewKeyValueStore(filePath, encryptionKey)
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
			_, err := kvStore.CompareAndSwap("key1", "value1", fmt.Sprintf("value%d", i), 0)
			if err != nil {
				t.Logf("CompareAndSwap error: %v", err)
			}
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

func TestCompressionAndEncryption(t *testing.T) {
	filePath := "test_compression_encryption.json"
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

	kvStore := store.NewKeyValueStore(filePath, encryptionKey)
	defer func() {
		kvStore.Stop()
		os.Remove(filePath)
	}()

	// Set a value and check if it can be retrieved correctly
	err := kvStore.Set("key1", "value1", 0)
	if err != nil {
		t.Fatalf("Failed to set key 'key1': %v", err)
	}

	value, err := kvStore.Get("key1")
	if err != nil {
		t.Fatalf("Failed to get key 'key1': %v", err)
	}
	if value != "value1" {
		t.Errorf("Expected value 'value1', got '%v'", value)
	}

	// Restart the KeyValueStore to ensure data is persisted and correctly loaded
	kvStore.Stop()
	kvStore = store.NewKeyValueStore(filePath, encryptionKey)

	value, err = kvStore.Get("key1")
	if err != nil {
		t.Fatalf("Failed to get key 'key1' after restart: %v", err)
	}
	if value != "value1" {
		t.Errorf("Expected value 'value1' after restart, got '%v'", value)
	}
}

func TestKeyValueStoreWithCompressionAndEncryption(t *testing.T) {
	filePath := "test_store_compression_encryption.json"
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

	kvStore := store.NewKeyValueStore(filePath, encryptionKey)
	defer func() {
		kvStore.Stop()
		os.Remove(filePath)
	}()

	// Test Set and Get operations
	err := kvStore.Set("name", "Jane", 0)
	if err != nil {
		t.Fatalf("Failed to set key 'name': %v", err)
	}

	value, err := kvStore.Get("name")
	if err != nil {
		t.Fatalf("Failed to get key 'name': %v", err)
	}
	expected := "Jane"
	if value != expected {
		t.Errorf("Expected value '%s', got '%v'", expected, value)
	}

	// Test Delete operation
	err = kvStore.Delete("name")
	if err != nil {
		t.Fatalf("Failed to delete key 'name': %v", err)
	}
	_, err = kvStore.Get("name")
	if err == nil {
		t.Fatalf("Expected error getting deleted key 'name'")
	}

	// Test CompareAndSwap operation
	err = kvStore.Set("key1", "value1", 0)
	if err != nil {
		t.Fatalf("Failed to set key 'key1': %v", err)
	}
	success, err := kvStore.CompareAndSwap("key1", "value1", "newValue", 0)
	if err != nil {
		t.Fatalf("Failed to compare and swap key 'key1': %v", err)
	}
	if !success {
		t.Fatalf("Expected CompareAndSwap to succeed for key 'key1'")
	}

	value, err = kvStore.Get("key1")
	if err != nil {
		t.Fatalf("Failed to get key 'key1' after compare and swap: %v", err)
	}
	if value != "newValue" {
		t.Errorf("Expected value 'newValue', got '%v'", value)
	}
}

func TestLargeDataCompressionAndEncryption(t *testing.T) {
	filePath := "test_large_data_compression_encryption.json"
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

	kvStore := store.NewKeyValueStore(filePath, encryptionKey)
	defer func() {
		kvStore.Stop()
		os.Remove(filePath)
	}()

	largeValue := make([]byte, 10*1024*1024) // 10 MB of data
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}

	err := kvStore.Set("largeKey", string(largeValue), 0)
	if err != nil {
		t.Fatalf("Failed to set large key: %v", err)
	}

	value, err := kvStore.Get("largeKey")
	if err != nil {
		t.Fatalf("Failed to get large key: %v", err)
	}
	if value != string(largeValue) {
		t.Errorf("Large value mismatch")
	}
}

func TestNonCompressibleData(t *testing.T) {
	filePath := "test_non_compressible_data.json"
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

	kvStore := store.NewKeyValueStore(filePath, encryptionKey)
	defer func() {
		kvStore.Stop()
		os.Remove(filePath)
	}()

	nonCompressibleValue := "abcdefghijklmnopqrstuvwxyz0123456789" // Example of non-compressible data

	err := kvStore.Set("nonCompressibleKey", nonCompressibleValue, 0)
	if err != nil {
		t.Fatalf("Failed to set non-compressible key: %v", err)
	}

	value, err := kvStore.Get("nonCompressibleKey")
	if err != nil {
		t.Fatalf("Failed to get non-compressible key: %v", err)
	}
	if value != nonCompressibleValue {
		t.Errorf("Non-compressible value mismatch")
	}
}

func TestHighlyCompressibleData(t *testing.T) {
	filePath := "test_highly_compressible_data.json"
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

	kvStore := store.NewKeyValueStore(filePath, encryptionKey)
	defer func() {
		kvStore.Stop()
		os.Remove(filePath)
	}()

	highlyCompressibleValue := strings.Repeat("a", 10000) // Highly compressible data

	err := kvStore.Set("highlyCompressibleKey", highlyCompressibleValue, 0)
	if err != nil {
		t.Fatalf("Failed to set highly compressible key: %v", err)
	}

	value, err := kvStore.Get("highlyCompressibleKey")
	if err != nil {
		t.Fatalf("Failed to get highly compressible key: %v", err)
	}
	if value != highlyCompressibleValue {
		t.Errorf("Highly compressible value mismatch")
	}
}

func TestNewDataFormat(t *testing.T) {
	filePath := "test_new_data_format.json"
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key for AES-256

	// Create new data format
	now := time.Now()
	newData := map[string][]store.KeyValue{
		"newKey": {{Value: "newValue", Timestamp: now}},
	}

	// Save new data format to file
	err := saveNewFormat(filePath, newData, encryptionKey)
	if err != nil {
		t.Fatalf("Failed to save new format data: %v", err)
	}

	// Initialize new KeyValueStore
	kvStore := store.NewKeyValueStore(filePath, encryptionKey)
	defer func() {
		kvStore.Stop()
		os.Remove(filePath)
	}()

	// Test if new data is readable
	value, err := kvStore.Get("newKey")
	if err != nil {
		t.Fatalf("Failed to get new key: %v", err)
	}

	expectedValue := "newValue"
	if value != expectedValue {
		t.Errorf("Expected value '%v', got '%v'", expectedValue, value)
	}
}

func saveNewFormat(filePath string, newData map[string][]store.KeyValue, encryptionKey []byte) error {
	data, err := json.Marshal(newData)
	if err != nil {
		return fmt.Errorf("error marshalling new format data: %v", err)
	}

	compressedData, err := store.CompressData(data)
	if err != nil {
		return fmt.Errorf("error compressing data: %v", err)
	}

	if len(encryptionKey) > 0 {
		encryptedData, err := store.EncryptData(compressedData, encryptionKey)
		if err != nil {
			return fmt.Errorf("error encrypting data: %v", err)
		}
		data = []byte(encryptedData)
	} else {
		data = compressedData
	}

	return os.WriteFile(filePath, data, 0644)
}
