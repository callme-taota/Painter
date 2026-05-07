package main

import (
	"log"

	"painter-2026/analytics-service/internal/transport/httpserver"
)

func main() {
	srv := httpserver.NewServer()
	if err := srv.Run(":18084"); err != nil {
		log.Fatal(err)
	}
}
