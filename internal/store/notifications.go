package store

import (
	"fmt"
	"log"
	"sync"
)

// NotificationManager manages the sending of store event notifications.
type NotificationManager struct {
	listeners []func(string)
	ch        chan string
	stopChan  chan struct{}
	mu        sync.Mutex
	wg        sync.WaitGroup
}

// NewNotificationManager creates a new NotificationManager.
func NewNotificationManager() *NotificationManager {
	nm := &NotificationManager{
		listeners: []func(string){},
		ch:        make(chan string, 10), // Buffer size for notifications
		stopChan:  make(chan struct{}),
	}

	go nm.listen()
	return nm
}

// RegisterListener registers a new listener for notifications.
func (nm *NotificationManager) RegisterListener(listener func(string)) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.listeners = append(nm.listeners, listener)
}

// UnregisterListener unregisters a listener for notifications.
func (nm *NotificationManager) UnregisterListener(listener func(string)) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	for i, l := range nm.listeners {
		if &l == &listener {
			nm.listeners = append(nm.listeners[:i], nm.listeners[i+1:]...)
			break
		}
	}
}

// Notify informe tous les écouteurs enregistrés d'un événement.
func (nm *NotificationManager) Notify(event string) {
	log.Printf("Notifying listeners: %s", event)
	nm.ch <- event
}

// NotifyAdd informe tous les écouteurs enregistrés de l'ajout d'une clé.
func (nm *NotificationManager) NotifyAdd(key string) {
	nm.Notify(fmt.Sprintf("added:%s", key))
}

// NotifyUpdate informe tous les écouteurs enregistrés de la mise à jour d'une clé.
func (nm *NotificationManager) NotifyUpdate(key string) {
	nm.Notify(fmt.Sprintf("updated:%s", key))
}

// NotifyDelete informe tous les écouteurs enregistrés de la suppression d'une clé.
func (nm *NotificationManager) NotifyDelete(key string) {
	nm.Notify(fmt.Sprintf("deleted:%s", key))
}

// listen écoute les événements et informe les écouteurs.
func (nm *NotificationManager) listen() {
	for {
		select {
		case event := <-nm.ch:
			nm.mu.Lock()
			for _, listener := range nm.listeners {
				// Retirer les goroutines pour garantir l'ordre des notifications
				listener(event)
			}
			nm.mu.Unlock()
		case <-nm.stopChan:
			return
		}
	}
}

// Stop arrête le gestionnaire de notification.
func (nm *NotificationManager) Stop() {
	close(nm.stopChan)
	nm.wg.Wait()
}
