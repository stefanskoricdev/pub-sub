# Simple Go Pub/Sub Server (Learning Example)

This project is a minimal, self-contained implementation of a Publish-Subscribe (Pub/Sub) messaging service built in **Go**. It uses Go's powerful concurrency primitives—**Goroutines**, **Channels**, and **Mutexes**—along with standard **HTTP** and **WebSockets** for network communication.

The primary goal of this repository is **learning and experimentation!**, please note it is not production-ready and lacks features like durable message persistence, scaling, backpressure management, advanced error handling etc...

---

## Overview

The service enables clients to subscribe to topics over **WebSocket connections** and receive real-time messages published to those topics.

- **Publishers** send messages to a specific topic via an HTTP endpoint.
- **Subscribers** connect via WebSockets and listen for messages on that topic.
- **Synchronization** is handled using `sync.RWMutex` to protect shared data while allowing concurrent operations.
- **Channels** ensure non-blocking message delivery between publishers and subscribers.

---

### Tri it out

1.  Run the server:

    ```bash
    go run main.go
    ```

The server will start on port `8080`.

2.  Connect Subscribers (WebSockets)

Open your browser console or a WebSocket client like Postman and connect:

```javascript
// Replace 'alerts' with the topic you want to test
const topic = "order_events";
const clientID = `client_${Math.floor(Math.random() * 1000)}`;

const ws = new WebSocket(`ws://localhost:8080/ws/sub/${topic}?id=${clientID}`);

ws.onopen = () =>
  console.log(`[${clientID}] Connected! Waiting for messages on '${topic}'.`);
ws.onmessage = (event) =>
  console.log(`[${clientID}] Received Message:`, JSON.parse(event.data));
ws.onclose = () => console.log(`[${clientID}] Connection closed.`);

// Connect a second client too
const ws2 = new WebSocket(
  `ws://localhost:8080/ws/sub/${topic}?id=second_client`
);
ws2.onmessage = (event) =>
  console.log(`[SECOND CLIENT] Received Message:`, JSON.parse(event.data));
```

3. Publish a Message (HTTP POST)

Use curl or any HTTP client to publish a message to the topic:

```bash
# Publish a message to the 'order_events' topic
curl -X POST http://localhost:8080/pub/order_events \
     -d "New Order: ORD-12345"
```

---
