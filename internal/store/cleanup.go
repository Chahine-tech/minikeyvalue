package store

import (
	"fmt"
	"log"
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
				log.Printf("Checking expiration for key %s at %v (expires at %v)", key, now, exp)
				if now.After(exp) {
					log.Printf("Key expired: %s", key) // Ajout de logs pour vérifier les expirations
					delete(kv.data, key)
					delete(kv.expirations, key)
					kv.notificationManager.Notify(fmt.Sprintf("expired:%s", key)) // Envoyer une notification d'expiration
				}
			}
			kv.Unlock()
		case <-kv.stopChan:
			log.Printf("Stopping cleanup goroutine") // Ajout de logs pour vérifier l'arrêt de la goroutine de nettoyage
			close(kv.cleanupStopped)
			return
		}
	}
}
