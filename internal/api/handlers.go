package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/Chahine-tech/minikeyvalue/internal/store"
)

var kvStore *store.KeyValueStore

// Initialize the KeyValueStore instance
func Initialize(store *store.KeyValueStore) {
	kvStore = store
}

func getKeyHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")

	value, err := kvStore.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if value == "" {
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(value)); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

func setKeyHandler(w http.ResponseWriter, r *http.Request) {
	var data map[string]string
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for key, value := range data {
		err = kvStore.Set(key, value, 0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write([]byte("Key set successfully")); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

func deleteKeyHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")

	err := kvStore.Delete(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	if _, err := w.Write([]byte("")); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}
