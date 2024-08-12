package store

// NotificationManager manages the sending of store event notifications.
type NotificationManager struct {
	listeners []func(string)
	ch        chan string
	stopChan  chan struct{}
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
	nm.listeners = append(nm.listeners, listener)
}

// Notify informs all registered listeners of an event.
func (nm *NotificationManager) Notify(key string) {
	nm.ch <- key
}

// listen listens to events and informs listeners.
func (nm *NotificationManager) listen() {
	for {
		select {
		case key := <-nm.ch:
			for _, listener := range nm.listeners {
				go listener(key)
			}
		case <-nm.stopChan:
			return
		}
	}
}

// Stop stops the notification manager.
func (nm *NotificationManager) Stop() {
	close(nm.stopChan)
}
