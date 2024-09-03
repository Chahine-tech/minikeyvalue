package main

import (
	"log"
	"os"
	"time"

	"github.com/Chahine-tech/minikeyvalue/internal/api"
	"github.com/Chahine-tech/minikeyvalue/internal/store"
)

func main() {
    // Set a default API key (insecure, don't do this in production)
    os.Setenv("API_KEY", "default_api_key")

    // Initialize the key-value store
    kvStore := store.NewKeyValueStore("data.json", []byte("my-secret-key"), 2*time.Minute, 1*time.Second)
    api.Initialize(kvStore)

    // Start the API server
    log.Println("Starting API server...")
    api.StartServer()
}
