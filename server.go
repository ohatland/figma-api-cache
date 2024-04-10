package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

var (
	cache      []byte
	cacheMutex sync.RWMutex
)

func fetchAndUpdateCache() {
	for {
		if err := godotenv.Load("secrets.env"); err != nil {
			log.Fatalf("Error loading .env file: %v", err)
		}

		FIGMA_API_URL := os.Getenv("FIGMA_API_URL")
		if FIGMA_API_URL == "" {
			log.Fatal("X_FIGMA_TOKEN is not set in secrets.env file")
		}

		FIGMA_API_TOKEN := os.Getenv("FIGMA_API_TOKEN")
		if FIGMA_API_TOKEN == "" {
			log.Fatal("X_FIGMA_TOKEN is not set in secrets.env file")
		}

		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		req, err := http.NewRequest("GET", FIGMA_API_URL, nil)
		if err != nil {
			log.Println("error creating request", err)
			continue
		}

		req.Header.Add("X-Figma-Token", FIGMA_API_TOKEN)

		resp, err := client.Do(req)
		if err != nil {
			log.Println("error fetching data from figma", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println("error reading response body", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("Received response from %s with status: %s\n", req.URL, resp.Status)
		}

		resp.Body.Close()

		cacheMutex.Lock()
		cache = body
		cacheMutex.Unlock()

		time.Sleep(1 * time.Minute)
	}
}

// Provide a read-only copy of the cache in response to API requests
func dataHandler(w http.ResponseWriter, r *http.Request) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(cache)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request: %s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		next.ServeHTTP(w, r)
	})
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	// Start a goroutine to fetch data from Figma and update the cache
	go fetchAndUpdateCache()

	r := mux.NewRouter()
	r.Use(loggingMiddleware)
	r.HandleFunc("/data", dataHandler).Methods("GET")

	// Start the server
	log.Println("Starting server on port 8080")
	http.ListenAndServe(":8080", r)
}
