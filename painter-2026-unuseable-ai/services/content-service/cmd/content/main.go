package main

import (
	"log"

	"painter-2026/content-service/internal/transport/httpserver"
)

func main() {
	srv := httpserver.NewServer()
	if err := srv.Run(":18082"); err != nil {
		log.Fatal(err)
	}
}
