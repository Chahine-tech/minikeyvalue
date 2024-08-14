package store

import (
	"fmt"
	"time"
)

// cleanupExpiredItems est une goroutine en arrière-plan qui vérifie périodiquement les éléments expirés et les supprime du magasin.
func (kv *KeyValueStore) cleanupExpiredItems(tickerInterval time.Duration) {
	ticker := time.NewTicker(tickerInterval)
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
					kv.notificationManager.Notify(fmt.Sprintf("expired:%s", key)) // Envoyer une notification d'expiration
				}
			}
			kv.Unlock()
		case <-kv.stopChan:
			close(kv.cleanupStopped)
			return
		}
	}
}
