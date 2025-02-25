package httpserver

import (
    "log"
    "net/http"
    "os"

    "gochatapp/pkg/redisrepo"

    "github.com/gorilla/mux"
    "github.com/rs/cors"
)

func StartHTTPServer() {
    // initialise redis
    redisClient := redisrepo.InitialiseRedis()
    defer redisClient.Close()

    // create indexes
    redisrepo.CreateFetchChatBetweenIndex()

    r := mux.NewRouter()

    // API routes - these need to come before the static file handler
    r.HandleFunc("/register", registerHandler).Methods(http.MethodPost)
    r.HandleFunc("/login", loginHandler).Methods(http.MethodPost)
    r.HandleFunc("/verify-contact", verifyContactHandler).Methods(http.MethodPost)
    r.HandleFunc("/chat-history", chatHistoryHandler).Methods(http.MethodGet)
    r.HandleFunc("/contact-list", contactListHandler).Methods(http.MethodGet)

    // Serve static files
    // Update this line in your StartHTTPServer function
    staticDir := "./client/build"  // Changed from "./client/public"
    log.Printf("Serving static files from: %s\n", staticDir)
    
    // Check if directory exists
    if _, err := os.Stat(staticDir); os.IsNotExist(err) {
        log.Printf("WARNING: Static directory %s does not exist!\n", staticDir)
    }
    
    // Create file server
    fs := http.FileServer(http.Dir(staticDir))
    
    // Register the file server as the handler for all other routes
    r.PathPrefix("/").Handler(fs)

    // Use default options for CORS
    handler := cors.Default().Handler(r)
    
    // Start the server
    log.Println("HTTP server starting on :8080")
    http.ListenAndServe(":8080", handler)
}