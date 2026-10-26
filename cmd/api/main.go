package main

import (
	"log"

	"github.com/joho/godotenv"

	"github.com/phamtuanha21092003/ecommerce-be/internal/domain/api"
)

func main() {
	_ = godotenv.Load("environment/.env.api")

	app, err := api.NewApp()
	if err != nil {
		log.Fatalf("start api: %v", err)
	}
	if err := app.Run(); err != nil {
		log.Fatalf("run api: %v", err)
	}
}
