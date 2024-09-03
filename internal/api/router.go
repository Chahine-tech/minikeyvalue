package api

import (
	"log"
	"net/http"
)

// RegisterRoutes registers API routes
func RegisterRoutes() {
	http.Handle("/api/v1/keys", AuthMiddleware(http.HandlerFunc(setKeyHandler), "admin"))
	http.Handle("/api/v1/keys", AuthMiddleware(http.HandlerFunc(getKeyHandler), "user"))
	http.Handle("/api/v1/keys", AuthMiddleware(http.HandlerFunc(deleteKeyHandler), "admin"))
}

// StartServer starts the HTTP server
func StartServer() {
	RegisterRoutes()
	log.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
