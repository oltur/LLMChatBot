package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	websiteURL := os.Getenv("WEBSITE_URL")
	if websiteURL == "" {
		log.Fatal("WEBSITE_URL environment variable is required")
	}

	ollamaService := NewOllamaService()
	scraper := NewWebScraper(ollamaService)
	chatbot := NewChatbot(scraper, ollamaService)
	server := NewServer(chatbot)

	r := mux.NewRouter()
	server.SetupRoutes(r)

	log.Printf("Target website: %s", websiteURL)

	if ollamaService.IsEnabled() {
		log.Println("Ollama CodeLlama integration enabled")
	} else {
		log.Println("Ollama integration disabled - ensure Ollama is running with codellama:13b model")
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
