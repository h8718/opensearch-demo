package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	opensearch "github.com/opensearch-project/opensearch-go"
)

type jsonResponse struct {
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

func main() {
	// Read OPENSEARCH_URL from environment variables
	opensearchURL := os.Getenv("OPENSEARCH_URL")
	if opensearchURL == "" {
		log.Fatal("OPENSEARCH_URL environment variable not set")
	}

	// Read PORT from environment variables (default to 8070 if not set)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8070"
	}

	// Initialize OpenSearch client
	client, err := opensearch.NewClient(opensearch.Config{
		Addresses: []string{opensearchURL},
	})
	if err != nil {
		log.Fatalf("Failed to create OpenSearch client: %v", err)
	}

	// Test connection to OpenSearch
	if !testOpenSearchConnection(client) {
		log.Fatal("Unable to connect to OpenSearch. Exiting.")
	}

	// Initialize chi router
	r := chi.NewRouter()

	// Add middlewares
	r.Use(middleware.Logger)                    // Log requests
	r.Use(middleware.Recoverer)                 // Recover from panics
	r.Use(middleware.Timeout(15 * time.Second)) // Set request timeout

	// Health Check Endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		sendJSONResponse(w, http.StatusOK, jsonResponse{Message: "API is healthy"})
	})

	// Root Endpoint
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		sendJSONResponse(w, http.StatusOK, jsonResponse{
			Message: fmt.Sprintf("Connected to OpenSearch at %s", opensearchURL),
		})
	})

	// Search Endpoint
	r.Get("/search", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query == "" {
			sendJSONResponse(w, http.StatusBadRequest, jsonResponse{
				Error: "Query parameter 'q' is required",
			})
			return
		}

		// Perform search in OpenSearch
		results, err := searchOpenSearch(client, query)
		if err != nil {
			sendJSONResponse(w, http.StatusInternalServerError, jsonResponse{
				Error: fmt.Sprintf("Search failed: %v", err),
			})
			return
		}

		sendJSONResponse(w, http.StatusOK, jsonResponse{
			Message: fmt.Sprintf("Search results for query: '%s'", query),
			Error:   results,
		})
	})

	// Create HTTP Server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      r,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	// Graceful Shutdown
	go func() {
		log.Printf("Starting server on :%s...\n", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %s", err)
		}
	}()

	// Wait for shutdown signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down server...")
	if err := srv.Close(); err != nil {
		log.Fatalf("Server failed to shut down: %s", err)
	}
	log.Println("Server stopped gracefully")
}

// testOpenSearchConnection tests the connection to OpenSearch by making a basic info request.
func testOpenSearchConnection(client *opensearch.Client) bool {
	for i := range 10 {
		time.Sleep(time.Duration(i+1*2) * time.Second)
		res, err := client.Info()
		if err != nil {
			log.Printf("Error getting OpenSearch info: %v\n", err)
			continue
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			log.Printf("Unexpected OpenSearch response code: %d\n", res.StatusCode)
			continue
		}

		log.Println("Successfully connected to OpenSearch")
		return true
	}
	return false
}

// searchOpenSearch performs a search query in OpenSearch and returns the results.
func searchOpenSearch(client *opensearch.Client, query string) (string, error) {
	searchBody := fmt.Sprintf(`{
		"query": {
			"match": {
				"content": "%s"
			}
		}
	}`, query)

	res, err := client.Search(
		client.Search.WithContext(context.Background()),
		client.Search.WithIndex("documents"),
		client.Search.WithBody(strings.NewReader(searchBody)),
	)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected response code: %d", res.StatusCode)
	}

	var resultMap map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&resultMap); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	results, _ := json.MarshalIndent(resultMap, "", "  ")
	return string(results), nil
}

// sendJSONResponse sends a JSON response with the specified HTTP status.
func sendJSONResponse(w http.ResponseWriter, status int, payload jsonResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, `{"error": "Internal Server Error"}`, http.StatusInternalServerError)
	}
}
