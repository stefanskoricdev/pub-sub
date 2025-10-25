package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Message interface{}

type Subscriber struct {
	id        string
	conn      *websocket.Conn
	channel   chan Message
	topicName string
}

type Topic struct {
	Name        string
	subscribers map[string]*Subscriber
	mu          sync.RWMutex
}

type PubSub struct {
	topics map[string]*Topic
	mu     sync.RWMutex
}

func NewPubSub() *PubSub {
	return &PubSub{
		topics: make(map[string]*Topic),
	}
}

func (pubSub *PubSub) getOrCreateTopic(topicName string) *Topic {
	pubSub.mu.Lock() // Acquire WRITE lock for PubSub service
	defer pubSub.mu.Unlock()

	topic, ok := pubSub.topics[topicName]

	if ok {
		return topic
	}

	newTopic := &Topic{
		Name:        topicName,
		subscribers: make(map[string]*Subscriber),
		// The RWMutex is initialized with 0 value
	}

	pubSub.topics[topicName] = newTopic

	return newTopic
}

func (topic *Topic) Unsubscribe(subscriberID string) {
	subscriber := topic.subscribers[subscriberID]

	close(subscriber.channel)
	delete(topic.subscribers, subscriberID)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Dev mode only
	},
}

func (pubSub *PubSub) Subscribe(w http.ResponseWriter, r *http.Request) error {
	topic := pubSub.getOrCreateTopic(r.PathValue("topic"))
	subscriberID := r.URL.Query().Get("id")

	log.Printf("Subscribed! Topic: %s, ID: %s", topic.Name, subscriberID)

	topic.mu.Lock() // Acquire WRITE lock for topic subscribers
	defer topic.mu.Unlock()

	// Check if subscriber already exists and if it does kill it
	// and create a new one. Also this should not happen. It means that on unsubscribe
	// we didn't do a cleanup. Log this too
	if _, exists := topic.subscribers[subscriberID]; exists {
		topic.Unsubscribe(subscriberID)
	}

	// Upgrade to WS connection
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		return fmt.Errorf("could not upgrade to websocket connection")
	}

	// Create subscriber
	sub := &Subscriber{
		id:        subscriberID,
		conn:      conn,
		channel:   make(chan Message, 64),
		topicName: topic.Name,
	}

	topic.subscribers[subscriberID] = sub

	// Start dedicated listener go routine for this subscriber.
	// This go routine handles all published messages to its topic
	// and sends them to client
	go sub.writer()

	return nil
}

func (wss *Subscriber) writer() {
	for message := range wss.channel {
		log.Println("Received message!")
		log.Println(message)

		// TO-DO send message to client through WS connection
	}
}

func (ps *PubSub) Publish(topicName string, message Message) error {
	ps.mu.RLock()
	topic, exists := ps.topics[topicName]
	defer ps.mu.RUnlock()

	if !exists {
		return fmt.Errorf("topic '%s' not found", topicName)
	}

	// Create dedicated goroutine for
	// each subscriber to fan out message
	for _, sub := range topic.subscribers {
		go func(s *Subscriber, m Message) {
			s.channel <- m
			log.Printf("Message sent: %s", m)
		}(sub, message)
	}

	return nil
}

var service = NewPubSub()

func handlePublish(w http.ResponseWriter, r *http.Request) {
	topicName := r.PathValue("topic")

	// Read the message body from the HTTP request.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	service.Publish(topicName, string(body))
}

func handleWSSubscribe(w http.ResponseWriter, r *http.Request) {
	if err := service.Subscribe(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	log.Println("Websocket server started at :8080")

	http.HandleFunc("POST /pub/{topic}", handlePublish)

	http.HandleFunc("GET /ws/sub/{topic}", handleWSSubscribe)

	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		log.Println("Error starting server:", err)
	}
}
