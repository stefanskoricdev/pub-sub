package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"simple-pub-sub/pubsub"
)

var service = pubsub.NewPubSubService()

func handlePublishRequest(w http.ResponseWriter, r *http.Request) {
	topicName := r.PathValue("topic")

	// Read the message body from the HTTP request.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %s", err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	if err := service.Publish(topicName, string(body)); err != nil {
		log.Printf("Error while publishing message: %s", err)
		http.Error(w, "Error while publishing message", http.StatusInternalServerError)
		return
	}
}

func handleWSSubscribeRequest(w http.ResponseWriter, r *http.Request) {
	if err := service.Subscribe(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	http.HandleFunc("POST /pub/{topic}", handlePublishRequest)

	http.HandleFunc("GET /ws/sub/{topic}", handleWSSubscribeRequest)

	srv := &http.Server{
		Addr: ":" + func() string {
			p := os.Getenv("PORT")
			if p == "" {
				return "8080"
			}
			return p
		}(),
	}

	go func() {
		log.Printf("Listening on %s\n", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Shutting down server gracefully…")
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shut down: %v", err)
	}
	log.Println("Server stopped")
}
