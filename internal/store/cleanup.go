package store

import (
	"time"
)

// cleanupExpiredItems is a background goroutine that periodically checks for expired items and removes them from the store.
func (kv *KeyValueStore) cleanupExpiredItems() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			kv.Lock()
			now := time.Now()
			for key, exp := range kv.expirations {
				if now.After(exp) {
					delete(kv.data, key)
					delete(kv.expirations, key)
				}
			}
			kv.Unlock()
		case <-kv.stopChan:
			close(kv.cleanupStopped)
			return
		}
	}
}
