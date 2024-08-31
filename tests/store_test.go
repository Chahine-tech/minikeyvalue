package main

import (
	"encoding/base64"
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

	// Ensure we clean up and persist data
	defer os.Remove(filePath)

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second
	// Initialize KeyValueStore
	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 1*time.Second, globalTTL)

	// Ensure we clean up and persist data
	defer kvStore.Stop()

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
	kvStore = store.NewKeyValueStore(filePath, encryptionKey, 1*time.Second, globalTTL)

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
	defer os.Remove(persistenceFile) // Supprimez le fichier après le test

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second

	kv := store.NewKeyValueStore(persistenceFile, encryptionKey, 1*time.Second, globalTTL)
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
	filePath := "test_kvstore_concurrency.json"
	defer os.Remove(filePath) // Remove the file after test

	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 2*time.Minute, 1*time.Second)
	defer kvStore.Stop()

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := kvStore.Set(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i), 0); err != nil {
				t.Errorf("error setting key 'key%d': %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			value, err := kvStore.Get(fmt.Sprintf("key%d", i))
			if err != nil || value != fmt.Sprintf("value%d", i) {
				t.Errorf("expected value 'value%d', got '%v' (error: %v)", i, value, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestCompareAndSwapConcurrency(t *testing.T) {
	filePath := "test_store_cas_concurrency.json"
	defer os.Remove(filePath) // Remove the file after test

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second

	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 1*time.Second, globalTTL)
	defer kvStore.Stop()

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
	defer os.Remove(filePath)

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second

	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 1*time.Second, globalTTL)
	defer kvStore.Stop()

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
	kvStore = store.NewKeyValueStore(filePath, encryptionKey, 1*time.Second, globalTTL)

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
	defer os.Remove(filePath)

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second

	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 1*time.Second, globalTTL)
	defer kvStore.Stop()

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
	defer os.Remove(filePath)

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second

	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 1*time.Second, globalTTL)
	defer kvStore.Stop()

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
	defer os.Remove(filePath)

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second

	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 1*time.Second, globalTTL)
	defer kvStore.Stop()

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
	defer os.Remove(filePath)

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second

	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 1*time.Second, globalTTL)
	defer kvStore.Stop()

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
	defer os.Remove(filePath)

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

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second

	// Initialize new KeyValueStore
	kvStore := store.NewKeyValueStore(filePath, encryptionKey, globalTTL, 1*time.Second)
	defer kvStore.Stop()

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
		// Encode the encrypted data to Base64
		data = []byte(base64.StdEncoding.EncodeToString(encryptedData))
	} else {
		// Encode the compressed data to Base64 if not encrypted
		data = []byte(base64.StdEncoding.EncodeToString(compressedData))
	}

	return os.WriteFile(filePath, data, 0644)
}

func TestGetVersion(t *testing.T) {
	filePath := "test_get_version.json"
	defer os.Remove(filePath)

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second
	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 1*time.Second, globalTTL)
	defer kvStore.Stop()

	// Add multiple versions
	err := kvStore.Set("key", "value1", 0)
	if err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}
	err = kvStore.Set("key", "value2", 0)
	if err != nil {
		t.Fatalf("Failed to set second value: %v", err)
	}
	err = kvStore.Set("key", "value3", 0)
	if err != nil {
		t.Fatalf("Failed to set third value: %v", err)
	}

	// Test
	v1, err := kvStore.GetVersion("key", 0)
	if err != nil || v1 != "value1" {
		t.Errorf("Expected value1 at version 0, got %s", v1)
	}

	v3, err := kvStore.GetVersion("key", 2)
	if err != nil || v3 != "value3" {
		t.Errorf("Expected value3 at version 2, got %s", v3)
	}

	// Test non-existent version
	_, err = kvStore.GetVersion("key", 3)
	if err == nil {
		t.Error("Expected error for non-existent version")
	}
}

func TestGetAllVersions(t *testing.T) {
	filePath := "test_get_all_versions.json"
	defer os.Remove(filePath)

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second
	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 1*time.Second, globalTTL)
	defer kvStore.Stop()

	// Add multiple versions
	err := kvStore.Set("key", "value1", 0)
	if err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}
	err = kvStore.Set("key", "value2", 0)
	if err != nil {
		t.Fatalf("Failed to set second value: %v", err)
	}
	err = kvStore.Set("key", "value3", 0)
	if err != nil {
		t.Fatalf("Failed to set third value: %v", err)
	}

	// Test retrieving all versions
	versions, err := kvStore.GetAllVersions("key")
	if err != nil {
		t.Fatalf("Failed to get all versions: %v", err)
	}
	if len(versions) != 3 {
		t.Errorf("Expected 3 versions, got %d", len(versions))
	}
	if versions[0] != "value1" || versions[1] != "value2" || versions[2] != "value3" {
		t.Errorf("Expected versions [value1, value2, value3], got %v", versions)
	}
}

func TestGetHistory(t *testing.T) {
	filePath := "test_get_history.json"
	defer os.Remove(filePath)

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second
	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 1*time.Second, globalTTL)
	defer kvStore.Stop()

	// Add multiple versions
	err := kvStore.Set("key", "value1", 0)
	if err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}
	time.Sleep(1 * time.Second)
	err = kvStore.Set("key", "value2", 0)
	if err != nil {
		t.Fatalf("Failed to set second value: %v", err)
	}
	time.Sleep(1 * time.Second)
	err = kvStore.Set("key", "value3", 0)
	if err != nil {
		t.Fatalf("Failed to set third value: %v", err)
	}

	// Test retrieving history
	history, err := kvStore.GetHistory("key")
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}
	if len(history) != 3 {
		t.Errorf("Expected 3 history entries, got %d", len(history))
	}
	if history[0].Value != "value1" || history[1].Value != "value2" || history[2].Value != "value3" {
		t.Errorf("Expected history values [value1, value2, value3], got %v", history)
	}
}
func TestRemoveVersion(t *testing.T) {
	filePath := "test_remove_version.json"
	defer os.Remove(filePath)

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second

	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 1*time.Second, globalTTL)
	defer kvStore.Stop()

	// Add multiple versions
	err := kvStore.Set("key", "value1", 0)
	if err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}
	err = kvStore.Set("key", "value2", 0)
	if err != nil {
		t.Fatalf("Failed to set second value: %v", err)
	}
	err = kvStore.Set("key", "value3", 0)
	if err != nil {
		t.Fatalf("Failed to set third value: %v", err)
	}

	// Remove the second version
	err = kvStore.RemoveVersion("key", 1)
	if err != nil {
		t.Fatalf("Failed to remove version 1: %v", err)
	}

	// Test retrieving remaining versions
	versions, err := kvStore.GetAllVersions("key")
	if err != nil {
		t.Fatalf("Failed to get all versions: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("Expected 2 versions, got %d", len(versions))
	}
	if versions[0] != "value1" || versions[1] != "value3" {
		t.Errorf("Expected versions [value1, value3], got %v", versions)
	}
}

func TestGetHistoryWithTimestamps(t *testing.T) {
	filePath := "test_get_history.json"
	defer os.Remove(filePath) // Delete the file after the test

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second
	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 1*time.Second, globalTTL)
	defer kvStore.Stop()

	// Add multiple versions
	err := kvStore.Set("key", "value1", 0)
	if err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}
	t1 := time.Now()
	time.Sleep(1 * time.Second)
	err = kvStore.Set("key", "value2", 0)
	if err != nil {
		t.Fatalf("Failed to set second value: %v", err)
	}
	t2 := time.Now()
	time.Sleep(1 * time.Second)
	err = kvStore.Set("key", "value3", 0)
	if err != nil {
		t.Fatalf("Failed to set third value: %v", err)
	}
	t3 := time.Now()

	// Test retrieving history
	history, err := kvStore.GetHistory("key")
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}
	if len(history) != 3 {
		t.Errorf("Expected 3 history entries, got %d", len(history))
	}
	if history[0].Value != "value1" || history[1].Value != "value2" || history[2].Value != "value3" {
		t.Errorf("Expected history values [value1, value2, value3], got %v", history)
	}

	// Validate timestamps are in correct order and close to expected times
	if !t1.Before(history[1].Timestamp) || !history[1].Timestamp.Before(t2) || !t2.Before(history[2].Timestamp) || !history[2].Timestamp.Before(t3) {
		t.Errorf("Timestamps are not in correct order or close to expected times")
	}
}

func TestKeyExpirationNotifications(t *testing.T) {
	filePath := "test_key_expiration.json"
	defer os.Remove(filePath)

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second
	kvStore := store.NewKeyValueStore(filePath, encryptionKey, globalTTL, 1*time.Second)
	defer kvStore.Stop()

	notifications := make([]string, 0)
	done := make(chan struct{})

	kvStore.RegisterNotificationListener(func(event string) {
		if len(event) > 0 && event[:8] == "expired:" {
			notifications = append(notifications, event[8:])
			if len(notifications) == 1 {
				close(done)
			}
		}
	})

	// Set the key with a 2 second expiry time
	err := kvStore.Set("temp-key", "temp-value", 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to set a key: %v", err)
	}

	// Wait long enough for expiration and notification to occur.
	select {
	case <-done:
		if len(notifications) != 1 || notifications[0] != "temp-key" {
			t.Errorf("Expected notification for 'temp-key', got %v", notifications)
		}
	case <-time.After(5 * time.Second):
		t.Errorf("Timeout waiting for key expiration notification")
	}
}

func TestMultipleNotifications(t *testing.T) {
	filePath := "test_multiple_notifications.json"
	defer os.Remove(filePath)

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second
	tickerInterval := 1 * time.Second
	kvStore := store.NewKeyValueStore(filePath, encryptionKey, tickerInterval, globalTTL)
	defer kvStore.Stop()

	notifications := make([]string, 0)
	done := make(chan struct{})

	kvStore.RegisterNotificationListener(func(event string) {
		log.Printf("Received notification: %s", event)
		notifications = append(notifications, event)
		if len(notifications) == 3 {
			close(done)
		}
	})

	// Add a key
	err := kvStore.Set("temp-key", "temp-value", 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to set a key: %v", err)
	}

	// Update the key

	err = kvStore.Set("temp-key", "new-value", 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to update the key: %v", err)
	}

	// Delete the key

	err = kvStore.Delete("temp-key")
	if err != nil {
		t.Fatalf("Failed to delete the key: %v", err)
	}

	select {
	case <-done:
		expectedNotifications := []string{"added:temp-key", "updated:temp-key", "deleted:temp-key"}
		for i, expected := range expectedNotifications {
			log.Printf("Checking notification: %s == %s ?", expected, notifications[i])
			if notifications[i] != expected {
				t.Errorf("Expected notification %s, got %s", expected, notifications[i])
			}
		}
	case <-time.After(10 * time.Second):
		t.Errorf("Timeout waiting for notifications")
	}
}

func TestGlobalTTL(t *testing.T) {
	const filePath = "test_global_ttl.json"
	defer os.Remove(filePath)

	globalTTL := 2 * time.Second
	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 1*time.Second, globalTTL)
	defer kvStore.Stop()

	err := kvStore.Set("key_with_global_ttl", "value", 0)
	if err != nil {
		t.Fatalf("Failed to set key: %v", err)
	}

	time.Sleep(globalTTL + 1*time.Second)

	_, err = kvStore.Get("key_with_global_ttl")
	if err == nil {
		t.Error("Expected key to be expired according to global TTL")
	}
}

func TestSetConcurrency(t *testing.T) {
	filePath := "test_set_concurrency.json"
	defer os.Remove(filePath) // Remove the file after test

	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 2*time.Minute, 1*time.Second)
	defer kvStore.Stop()

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := kvStore.Set(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i), 0); err != nil {
				t.Errorf("error setting key 'key%d': %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < 100; i++ {
		value, err := kvStore.Get(fmt.Sprintf("key%d", i))
		if err != nil {
			t.Errorf("expected key 'key%d' to be present, got error: %v", i, err)
		}
		if value != fmt.Sprintf("value%d", i) {
			t.Errorf("expected value 'value%d', got '%v'", i, value)
		}
	}
}

func TestGetSetConcurrency(t *testing.T) {
	filePath := "test_get_set_concurrency.json"
	defer os.Remove(filePath) // Remove the file after test

	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 2*time.Minute, 1*time.Second)
	defer kvStore.Stop()

	var setWG sync.WaitGroup
	var getWG sync.WaitGroup

	for i := 0; i < 100; i++ {
		setWG.Add(1)
		go func(i int) {
			defer setWG.Done()
			if err := kvStore.Set(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i), 0); err != nil {
				t.Errorf("error setting key 'key%d': %v", i, err)
			}
		}(i)
	}

	for i := 0; i < 100; i++ {
		getWG.Add(1)
		go func(i int) {
			defer getWG.Done()
			if _, err := kvStore.Get(fmt.Sprintf("key%d", i)); err != nil && err.Error() != "key not found" {
				t.Errorf("error getting key 'key%d': %v", i, err)
			}
		}(i)
	}

	setWG.Wait()
	getWG.Wait()
}

func TestDeleteConcurrency(t *testing.T) {
	filePath := "test_delete_concurrency.json"
	defer os.Remove(filePath) // Remove the file after test

	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 2*time.Minute, 1*time.Second)
	defer kvStore.Stop()

	var setWG sync.WaitGroup
	var delWG sync.WaitGroup

	for i := 0; i < 100; i++ {
		setWG.Add(1)
		go func(i int) {
			defer setWG.Done()
			if err := kvStore.Set(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i), 0); err != nil {
				t.Errorf("error setting key 'key%d': %v", i, err)
			}
		}(i)
	}
	setWG.Wait()

	for i := 0; i < 100; i++ {
		delWG.Add(1)
		go func(i int) {
			defer delWG.Done()
			if err := kvStore.Delete(fmt.Sprintf("key%d", i)); err != nil && err.Error() != "key not found" {
				t.Errorf("error deleting key 'key%d': %v", i, err)
			}
		}(i)
	}
	delWG.Wait()

	for i := 0; i < 100; i++ {
		_, err := kvStore.Get(fmt.Sprintf("key%d", i))
		if err == nil {
			t.Errorf("expected key 'key%d' to be deleted, but it still exists", i)
		}
	}
}

func TestLazyLoading(t *testing.T) {
	filePath := "test_lazy_loading.json"
	defer os.Remove(filePath)

	kvStore := store.NewKeyValueStore(filePath, encryptionKey, 2*time.Minute, 1*time.Second)
	defer kvStore.Stop()

	if kvStore.Loaded() {
		t.Fatalf("Data should not be loaded initially")
	}

	// Test Get operation triggers loading
	_, err := kvStore.Get("nonexistent")
	if err == nil || !strings.Contains(err.Error(), "key not found") {
		t.Fatalf("Expected error getting nonexistent key, got %v", err)
	}

	if !kvStore.Loaded() {
		t.Fatalf("Data should be loaded after a Get operation")
	}

	// Reset for Set operation test
	kvStore = store.NewKeyValueStore(filePath, encryptionKey, 2*time.Minute, 1*time.Second)
	defer kvStore.Stop()

	if kvStore.Loaded() {
		t.Fatalf("Data should not be loaded initially")
	}

	err = kvStore.Set("key1", "value1", 0)
	if err != nil {
		t.Fatalf("Failed to set key: %v", err)
	}

	if !kvStore.Loaded() {
		t.Fatalf("Data should be loaded after a Set operation")
	}
}

func TestKeyRotation(t *testing.T) {
	filePath := "test_key_rotation.json"
	defer os.Remove(filePath)

	originalKey := []byte("originalkey01234")
	newKey := []byte("newkey0123456789")

	// Create store with original key
	kvStore := store.NewKeyValueStore(filePath, originalKey, 2*time.Minute, 1*time.Second)
	log.Println("Store created with original key")

	// Set a key
	err := kvStore.Set("key1", "value1", 0)
	if err != nil {
		t.Fatalf("Failed to set key: %v", err)
	}
	log.Println("Key set: key1 -> value1")

	// Rotate key
	err = kvStore.RotateEncryptionKey(newKey)
	if err != nil {
		t.Fatalf("Failed to rotate encryption key: %v", err)
	}
	log.Println("Key rotated successfully")

	// Get key with new encryption
	value, err := kvStore.Get("key1")
	if err != nil {
		t.Fatalf("Failed to get key after rotation: %v", err)
	}
	if value != "value1" {
		t.Errorf("Expected value 'value1', got '%v'", value)
	}
	log.Println("Key retrieved after rotation: key1 ->", value)

	// Stop the store
	kvStore.Stop()
	log.Println("Store stopped successfully")

	// Create a new store with the new key
	newStore := store.NewKeyValueStore(filePath, newKey, 2*time.Minute, 1*time.Second)
	log.Println("New store created with new key")

	// Try to get the key
	value, err = newStore.Get("key1")
	if err != nil {
		t.Fatalf("Failed to get key after restart: %v", err)
	}
	if value != "value1" {
		t.Errorf("Expected value 'value1' after restart, got '%v'", value)
	}
	log.Println("Key retrieved after restart: key1 ->", value)

	// Stop the new store
	newStore.Stop()
	log.Println("New store stopped successfully")
}

func TestKeyRotationWithIncorrectOldKey(t *testing.T) {
	filePath := "test_incorrect_old_key.json"
	defer os.Remove(filePath)

	originalKey := []byte("originalkey01234")
	newKey := []byte("newkey0123456789")

	// Create store with original key and set data
	kvStore := store.NewKeyValueStore(filePath, originalKey, 2*time.Minute, 1*time.Second)
	err := kvStore.Set("key1", "value1", 0)
	if err != nil {
		t.Fatalf("Failed to set key: %v", err)
	}
	kvStore.Stop()

	// Debugging: Verify data was encrypted with the correct key
	initialData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fmt.Printf("Initial encrypted data: %x\n", initialData)

	// Re-open store using the filePath to load potential existing key
	kvStore = store.NewKeyValueStore(filePath, nil, 2*time.Minute, 1*time.Second)

	// Now attempt key rotation - this should fail
	err = kvStore.RotateEncryptionKey(newKey)
	if err == nil {
		t.Fatalf("Expected error when rotating encryption key with incorrect old key, got none")
	}
	if !strings.Contains(err.Error(), "failed to decrypt data with old key") {
		t.Fatalf("Expected error message to contain 'failed to decrypt data with old key', got: %v", err)
	}

	kvStore.Stop()
}
