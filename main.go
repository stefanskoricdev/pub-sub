package main

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Message interface{}

type Subscriber struct {
	id        string
	conn      *websocket.Conn
	send      chan Message
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

func (pubSub *PubSub) getTopic(topicName string) *Topic {
	pubSub.mu.Lock() // Acquire WRITE lock for PubSub service
	defer pubSub.mu.Unlock()

	topic, ok := pubSub.topics[topicName]

	if ok {
		return topic
	}

	return &Topic{
		Name:        topicName,
		subscribers: make(map[string]*Subscriber),
		// The RWMutex is initialized with 0 value
	}
}

func (topic *Topic) Unsubscribe(subscriberID string) {
	subscriber := topic.subscribers[subscriberID]

	close(subscriber.send)
	delete(topic.subscribers, subscriberID)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Dev mode only
	},
}

func (pubSub *PubSub) Subscribe(w http.ResponseWriter, r *http.Request) error {
	topic := pubSub.getTopic(r.PathValue("topic"))
	subscriberID := r.URL.Query().Get("id")

	topic.mu.Lock() // Acquire WRITE lock for topic subscribers
	defer topic.mu.Unlock()

	// Check if subscriber already exists and if it does kill it
	// and create a new one. Also this should not happen. It means that on unsubscribe
	// we didn't do a cleanup. Log this too
	if _, exists := topic.subscribers[subscriberID]; exists {
		topic.Unsubscribe(subscriberID)
	}

	// Create ws connection
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		return fmt.Errorf("could not upgrade to websocket connection")
	}
	// Create channel

	topic.subscribers[subscriberID] = &Subscriber{
		id:        subscriberID,
		conn:      conn,
		send:      make(chan Message, 64),
		topicName: topic.Name,
	}

	return nil
}

func handleWSSubscribe(w http.ResponseWriter, r *http.Request) {
	if err := service.Subscribe(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var service = NewPubSub()

func main() {
	fmt.Println("Websocket server started at :8080")

	http.HandleFunc("/ws/subscribe/{topic}", handleWSSubscribe)

	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
