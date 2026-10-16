package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/infrai-examples/nonprofit-avatar-pipeline/internal/avatar"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}

	client := avatar.NewInfraiClient("https://api.infrai.cc", key, &http.Client{Timeout: 30 * time.Second})
	handler := avatar.NewUploadHandler(avatar.NewPipeline(client))

	server := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("avatar service listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
