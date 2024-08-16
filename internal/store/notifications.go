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

// Notify informs all registered listeners of an event.
func (nm *NotificationManager) Notify(event string) {
	log.Printf("Notifying listeners: %s", event)
	nm.ch <- event
}

// NotifyAdd informs all registered listeners that a key has been added.
func (nm *NotificationManager) NotifyAdd(key string) {
	nm.Notify(fmt.Sprintf("added:%s", key))
}

// NotifyUpdate informs all registered headphones when a key is updated.
func (nm *NotificationManager) NotifyUpdate(key string) {
	nm.Notify(fmt.Sprintf("updated:%s", key))
}

// NotifyDelete informs all registered listeners that a key has been deleted.
func (nm *NotificationManager) NotifyDelete(key string) {
	nm.Notify(fmt.Sprintf("deleted:%s", key))
}

// listen listens to events and informs listeners.
func (nm *NotificationManager) listen() {
	for {
		select {
		case event := <-nm.ch:
			nm.mu.Lock()
			for _, listener := range nm.listeners {
				// Remove goroutines to guarantee notification order
				listener(event)
			}
			nm.mu.Unlock()
		case <-nm.stopChan:
			return
		}
	}
}

// Stop stops the notification manager.
func (nm *NotificationManager) Stop() {
	close(nm.stopChan)
	nm.wg.Wait()
}
