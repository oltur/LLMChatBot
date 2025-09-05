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

	scraper := NewWebScraper()
	ollamaService := NewOllamaService()
	chatbot := NewChatbot(scraper, ollamaService)
	server := NewServer(chatbot)

	r := mux.NewRouter()
	server.SetupRoutes(r)

	if ollamaService.IsEnabled() {
		log.Println("Ollama CodeLlama integration enabled")
	} else {
		log.Println("Ollama integration disabled - ensure Ollama is running with codellama:13b model")
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
