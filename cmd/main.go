package main

import (
	"log"
	"time"

	"github.com/Chahine-tech/minikeyvalue/internal/store"
)

// Example demonstrates how to use the KeyValueStore.
func main() {
	filePath := "data.json"
	encryptionKey := []byte("encryptionKey")

	// Set a global TTL of 10 seconds.
	globalTTL := 10 * time.Second

	kv := store.NewKeyValueStore(filePath, encryptionKey, 5*time.Second, globalTTL)
	defer kv.Stop()

	err := kv.Set("key1", "value1", 0)
	if err != nil {
		log.Fatalf("Error setting value: %v", err)
	}

	value, err := kv.Get("key1")
	if err != nil {
		log.Fatalf("Error getting value: %v", err)
	}
	log.Printf("Retrieved value: %v\n", value)

	time.Sleep(5 * time.Second)
}
