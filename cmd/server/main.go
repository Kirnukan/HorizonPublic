package main

import (
	"HorizonBackend/config"
	"HorizonBackend/internal/router"
	"HorizonBackend/scripts"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"
)

func setCORSHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if r.Method == "OPTIONS" {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	db, err := config.NewConnection(cfg)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			fmt.Println("Error closing the database:", err)
		}
	}()

	// Этот скрипт добавляет изображения из папки в базу данных
	scripts.AddImagesFromFolder(db, "./static/images")

	r := router.NewRouter(db, cfg)

	fmt.Println("Server started on :8000")
	err = http.ListenAndServe(":8000", setCORSHeaders(r))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
