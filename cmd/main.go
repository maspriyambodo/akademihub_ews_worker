package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"

	"github.com/sekolahpintar/ews-worker/internal/config"
	"github.com/sekolahpintar/ews-worker/internal/db"
	"github.com/sekolahpintar/ews-worker/internal/handler"
	"github.com/sekolahpintar/ews-worker/internal/middleware"
	"github.com/sekolahpintar/ews-worker/internal/repository"
	"github.com/sekolahpintar/ews-worker/internal/service"
)

func main() {
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			log.Printf("warning: could not load .env: %v", err)
		}
	}

	cfg := config.Load()

	database, err := db.New(cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer database.Close()

	log.Println("database connected")

	// Repositories
	ewsRepo := repository.NewEWSRepo(database)
	siswaRepo := repository.NewSiswaRepo(database)

	// Services
	ewsSvc := service.NewEWSService(ewsRepo, siswaRepo)

	// Handlers
	ewsH := handler.NewEWSHandler(ewsSvc)

	// Router
	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"ews-worker"}`))
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWTSecret, database))

		r.Route("/api/v1/ews", func(r chi.Router) {
			r.Post("/process", ewsH.Process)
			r.Post("/process-siswa/{siswaId}", ewsH.ProcessSiswa)
			r.Get("/alerts", ewsH.ListAlerts)
			r.Get("/alerts/{siswaId}", ewsH.GetAlertsBySiswa)
			r.Patch("/alerts/{id}/resolve", ewsH.ResolveAlert)
			r.Get("/{id}", ewsH.GetAlert)
		})
	})

	addr := ":" + cfg.AppPort
	log.Printf("ews-worker listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
