package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
)

type Server struct {
	chatbot *Chatbot
}

type ChatRequest struct {
	Message string `json:"message"`
}

type ChatResponse struct {
	Response  string `json:"response"`
	Timestamp string `json:"timestamp"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func NewServer(chatbot *Chatbot) *Server {
	return &Server{
		chatbot: chatbot,
	}
}

func (s *Server) SetupRoutes(r *mux.Router) {
	r.HandleFunc("/", s.serveIndex).Methods("GET")
	r.HandleFunc("/chat", s.handleChat).Methods("POST")
	r.HandleFunc("/health", s.handleHealth).Methods("GET")

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join("static", "index.html"))
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Invalid JSON format"})
		return
	}

	if req.Message == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Message cannot be empty"})
		return
	}

	chatMessage, err := s.chatbot.ProcessMessage(req.Message)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Failed to process message"})
		return
	}

	response := ChatResponse{
		Response:  chatMessage.Response,
		Timestamp: chatMessage.Timestamp.Format("2006-01-02 15:04:05"),
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}
