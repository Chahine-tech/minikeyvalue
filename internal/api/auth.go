package api

import (
	"log"
	"net/http"
	"os"
)

var roles = map[string]string{
	"default_api_key": "admin",
	"read_only_key":   "user",
}

// AuthMiddleware handles API key authentication
func AuthMiddleware(next http.Handler, requiredRole string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		validAPIKey := os.Getenv("API_KEY")
		userRole, exists := roles[apiKey]

		if !exists || apiKey != validAPIKey || !hasRole(userRole, requiredRole) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		log.Printf("Authorized access by apiKey: %s, role: %s", apiKey, userRole)

		// Valid API key and role, continue to next handler
		next.ServeHTTP(w, r)
	})
}

func hasRole(userRole, requiredRole string) bool {
	if requiredRole == "admin" {
		return userRole == "admin"
	}
	if requiredRole == "user" {
		return userRole == "admin" || userRole == "user" // Admins can do user tasks
	}
	return false
}
