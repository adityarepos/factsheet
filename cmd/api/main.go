package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"factsheet/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver (pgx stdlib)
)

func main() {
	// 1. Connect to Database (Ensure DATABASE_URL is set in your terminal)
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Successfully connected to Neon PostgreSQL!")

	// 2. Initialize Repository
	repo := repository.NewFactsheetRepo(db)

	// 3. Set up Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	
	// 1. Serves your ultra-premium frontend index.html dashboard at the root domain
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./index.html")
	})

	// 2. Serves your core quantitative JSON backend data endpoint
	// Clean, pragmatic route matching the Prisma Global Growth Factsheet
	r.Get("/api/factsheet", func(w http.ResponseWriter, r *http.Request) {
		// Hardcoded to 1 since this backend explicitly powers the Prisma portfolio
		defaultPortfolioID := 1

		// Fetch the aggregated factsheet data from the repository layer
		factsheet, err := repo.GetFactsheet(r.Context(), defaultPortfolioID)
		if err != nil {
			log.Printf("Error fetching factsheet: %v", err)
			http.Error(w, "Failed to load factsheet", http.StatusInternalServerError)
			return
		}

		// Return clean JSON response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(factsheet)
	})

	// 4. Start Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	
	log.Printf("Server starting on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}