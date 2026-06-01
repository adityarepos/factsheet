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

	// TODO: Ensure this matches your go.mod module name
	"factsheet/internal/repository"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("CRITICAL: DATABASE_URL environment variable is missing")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("CRITICAL: Unable to establish database connection pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("CRITICAL: Database ping failed: %v", err)
	}

	// CRITICAL FIX: Global Repository Initialization
	repo := repository.NewFactsheetRepository(pool)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Expose the X-Cache header to the browser
			w.Header().Set("Access-Control-Expose-Headers", "X-Cache")
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

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./index.html")
	})

	r.Get("/api/factsheet", func(w http.ResponseWriter, r *http.Request) {
		defaultPortfolioID := 1

		// Notice the boolean catch
		factsheet, isCached, err := repo.GetFactsheet(r.Context(), defaultPortfolioID)
		if err != nil {
			log.Printf("ERROR: Failed to pull portfolio metrics: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Inject Truth Header
		if isCached {
			w.Header().Set("X-Cache", "HIT")
		} else {
			w.Header().Set("X-Cache", "MISS")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(factsheet)
	})

	log.Printf("[SUCCESS] REST Engine deployed on Port %s...", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("CRITICAL: Server crashed: %v", err)
	}
}