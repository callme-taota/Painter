package main

import (
	"log"

	"painter-2026/api-gateway/internal/transport/httpserver"
)

func main() {
	srv := httpserver.NewServer()
	if err := srv.Run(":18080"); err != nil {
		log.Fatal(err)
	}
}
