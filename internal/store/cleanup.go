package store

import (
	"log"
	"time"
)

// cleanupExpiredItems is a background goroutine that periodically checks for expired items and removes them from the store.
func (kv *KeyValueStore) cleanupExpiredItems() {
	ticker := time.NewTicker(1 * time.Second) // Reduce test interval
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			kv.Lock()
			now := time.Now()
			for key, exp := range kv.expirations {
				log.Printf("Checking expiration for key %s", key) // Add logs to view verifications
				if now.After(exp) {
					delete(kv.data, key)
					delete(kv.expirations, key)
					log.Printf("Key expired: %s", key) // Add logs to see expirations
					kv.notificationManager.Notify(key) //Send expiry notification
				}
			}
			kv.Unlock()
		case <-kv.stopChan:
			close(kv.cleanupStopped)
			return
		}
	}
}
