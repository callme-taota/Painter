package main

import (
	"log"

	"painter-2026/identity-service/internal/transport/httpserver"
)

func main() {
	srv := httpserver.NewServer()
	if err := srv.Run(":18081"); err != nil {
		log.Fatal(err)
	}
}
