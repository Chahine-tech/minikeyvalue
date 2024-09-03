package api

import (
	"log"
	"net/http"
)

// RegisterRoutes registers API routes
func RegisterRoutes() {
	http.Handle("/api/v1/data", AuthMiddleware(http.HandlerFunc(dataHandler), "user")) // changed from http.HandleFunc to http.Handle
}

// StartServer starts the HTTP server
func StartServer() {
	RegisterRoutes()
	log.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}

// Sample handler
func dataHandler(w http.ResponseWriter, r *http.Request) {
	// This is a stub. You should replace this with actual data handling logic.
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Data endpoint"))
}
