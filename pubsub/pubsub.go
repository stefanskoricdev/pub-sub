package pubsub

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	TopicName string
	Payload   string
}

type ResponseMessage struct {
	TopicName string `json:"topicName"`
	Payload   string `json:"payload"`
	Time      string `json:"time"`
}

type WebSocketSubscriber struct {
	ID        string
	Conn      *websocket.Conn
	Channel   chan Message
	TopicName string
}

type Topic struct {
	Name        string
	Subscribers map[string]*WebSocketSubscriber
	Mu          sync.RWMutex
}

type PubSubService struct {
	Topics map[string]*Topic
	Mu     sync.RWMutex
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Dev mode only
	},
}

func (pss *PubSubService) GetOrCreateTopic(topicName string) *Topic {
	pss.Mu.Lock() // Acquire WRITE lock for PubSubService Service
	defer pss.Mu.Unlock()

	topic, ok := pss.Topics[topicName]

	if ok {
		return topic
	}

	newTopic := &Topic{
		Name:        topicName,
		Subscribers: make(map[string]*WebSocketSubscriber),
		// 	Mu: The RWMutex is initialized with 0 value
	}

	pss.Topics[topicName] = newTopic

	return newTopic
}

func NewPubSubService() *PubSubService {
	return &PubSubService{
		Topics: make(map[string]*Topic),
	}
}

func (pss *PubSubService) Unsubscribe(topicName string, subscriberID string) {
	pss.Mu.RLock()
	topic, ok := pss.Topics[topicName]
	pss.Mu.RUnlock()

	if !ok {
		return
	}

	topic.Mu.Lock()
	defer topic.Mu.Unlock()

	sub, exists := topic.Subscribers[subscriberID]

	if !exists {
		return
	}

	// Remove subscriber from a map
	delete(topic.Subscribers, subscriberID)

	// Close internal queue channel. This signals the sub.WritePump() goroutine to exit.
	close(sub.Channel)
}

func (pss *PubSubService) Subscribe(w http.ResponseWriter, r *http.Request) error {
	topic := pss.GetOrCreateTopic(r.PathValue("topic"))
	subscriberID := r.URL.Query().Get("id")

	if subscriberID == "" {
		return fmt.Errorf("query parameter 'id' missing")
	}

	topic.Mu.Lock() // Acquire WRITE lock for topic subscribers
	defer topic.Mu.Unlock()

	// Check if subscriber already exists and if it does kill it
	// and create a new one. Also this should not happen. It means that on unsubscribe
	// we didn't do a cleanup.
	if _, exists := topic.Subscribers[subscriberID]; exists {
		pss.Unsubscribe(topic.Name, subscriberID)
	}

	// Upgrade to WS connection
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		return fmt.Errorf("could not upgrade to websocket connection")
	}

	// Create subscriber
	webSocketSubsriber := &WebSocketSubscriber{
		ID:        subscriberID,
		Conn:      conn,
		Channel:   make(chan Message, 64),
		TopicName: topic.Name,
	}

	topic.Subscribers[subscriberID] = webSocketSubsriber

	// Start dedicated listener go routine for this subscriber.
	// This go routine handles all published messages to its topic
	// and sends them to client
	go webSocketSubsriber.WritePump(pss)

	return nil
}

func (wss *WebSocketSubscriber) WritePump(pss *PubSubService) {
	// Ensure we cleanup when this goroutine exits
	defer func() {
		pss.Unsubscribe(wss.TopicName, wss.ID)
	}()

	for m := range wss.Channel {
		messageJson, err := json.Marshal(ResponseMessage{
			TopicName: m.TopicName,
			Payload:   m.Payload,
			Time:      time.Now().Format(time.RFC3339),
		})

		if err != nil {
			log.Printf("Error marshalling message for %s: %v", wss.TopicName, err)
			wss.Conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		if err := wss.Conn.WriteMessage(websocket.TextMessage, []byte(messageJson)); err != nil {
			log.Printf("Error while sending message to subscriber, %s", err)
			return
		}
	}
}

func (pss *PubSubService) Publish(topicName string, message string) error {
	pss.Mu.RLock()
	topic, exists := pss.Topics[topicName]
	defer pss.Mu.RUnlock()

	if !exists {
		return fmt.Errorf("topic '%s' not found", topicName)
	}

	// Create dedicated goroutine for
	// each subscriber to fan out message
	for _, sub := range topic.Subscribers {
		go func(s *WebSocketSubscriber, m Message) {
			s.Channel <- m
		}(sub, Message{
			TopicName: topicName,
			Payload:   message,
		})
	}

	return nil
}
