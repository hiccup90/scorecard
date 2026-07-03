package main

import (
	"log"
	"net/http"

	"github.com/hiccup90/scorecard/internal/config"
	"github.com/hiccup90/scorecard/internal/database"
	"github.com/hiccup90/scorecard/internal/server"
)

func main() {
	cfg := config.Load()
	db, err := database.Open(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	srv := server.New(cfg, db)
	log.Printf("Scorecard running on http://localhost%s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
