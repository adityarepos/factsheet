package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	// TODO: Replace "factsheet" with the exact module name defined in your go.mod file
	"factsheet/internal/repository"
)

func main() {
	// 1. Fetch Cloud Environment Variables
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("CRITICAL: DATABASE_URL environment variable is missing")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// 2. Establish High-Performance Serverless Connection Pool to Neon PostgreSQL
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("CRITICAL: Unable to establish database connection pool: %v", err)
	}
	defer pool.Close()

	// Test Connection Health
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("CRITICAL: Database ping failed: %v", err)
	}

	// 3. CRITICAL CACHE FIX: Initialize the repository EXACTLY ONCE globally.
	// This ensures your internal map and sync.RWMutex states are preserved in memory 
	// across all concurrent HTTP routing threads.
	repo := repository.NewFactsheetRepository(pool)

	// 4. Set up Router Configuration
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Global Explicit Cross-Origin Resource Sharing (CORS) Middleware Layer
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// 5. Interface Endpoints Setup
	
	// Serve the clean dark-mode visual web dashboard at the root path
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./index.html")
	})

	// Serve the core unified JSON payload
	r.Get("/api/factsheet", func(w http.ResponseWriter, r *http.Request) {
		defaultPortfolioID := 1

		// Accesses the globally scoped, stateful repository instance directly
		factsheet, err := repo.GetFactsheet(r.Context(), defaultPortfolioID)
		if err != nil {
			log.Printf("ERROR: Failed to pull portfolio factsheet metrics: %v", err)
			http.Error(w, "Internal Server Error: Failed to load portfolio definitions", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(factsheet)
	})

	log.Printf("[SUCCESS] REST Engine deployed. Routing connections actively on Port %s...", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("CRITICAL: Server crashed during runtime execution: %v", err)
	}
}