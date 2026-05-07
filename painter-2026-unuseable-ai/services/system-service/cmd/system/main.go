package main

import (
	"log"

	"painter-2026/system-service/internal/transport/httpserver"
)

func main() {
	srv := httpserver.NewServer()
	if err := srv.Run(":18083"); err != nil {
		log.Fatal(err)
	}
}
