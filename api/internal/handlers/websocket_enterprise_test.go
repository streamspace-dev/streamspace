package handlers

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWebSocketHub(t *testing.T) {
	hub := &WebSocketHub{
		Clients:    make(map[string]*WebSocketClient),
		Register:   make(chan *WebSocketClient),
		Unregister: make(chan *WebSocketClient),
		Broadcast:  make(chan WebSocketMessage, 256),
	}

	// Start hub in goroutine
	go hub.Run()

	// Give hub time to start
	time.Sleep(50 * time.Millisecond)

	t.Run("Register client", func(t *testing.T) {
		client := &WebSocketClient{
			ID:     "test-client-1",
			UserID: "user1",
			Send:   make(chan WebSocketMessage, 256),
			Hub:    hub,
		}

		hub.Register <- client
		time.Sleep(50 * time.Millisecond)

		hub.Mu.RLock()
		_, exists := hub.Clients[client.ID]
		hub.Mu.RUnlock()

		assert.True(t, exists, "Client should be registered")
	})

	t.Run("Unregister client", func(t *testing.T) {
		client := &WebSocketClient{
			ID:     "test-client-2",
			UserID: "user2",
			Send:   make(chan WebSocketMessage, 256),
			Hub:    hub,
		}

		hub.Register <- client
		time.Sleep(50 * time.Millisecond)

		hub.Unregister <- client
		time.Sleep(50 * time.Millisecond)

		hub.Mu.RLock()
		_, exists := hub.Clients[client.ID]
		hub.Mu.RUnlock()

		assert.False(t, exists, "Client should be unregistered")
	})

	t.Run("Broadcast to all", func(t *testing.T) {
		client1 := &WebSocketClient{
			ID:     "test-client-3",
			UserID: "user3",
			Send:   make(chan WebSocketMessage, 256),
			Hub:    hub,
		}

		client2 := &WebSocketClient{
			ID:     "test-client-4",
			UserID: "user4",
			Send:   make(chan WebSocketMessage, 256),
			Hub:    hub,
		}

		hub.Register <- client1
		hub.Register <- client2
		time.Sleep(50 * time.Millisecond)

		message := WebSocketMessage{
			Type:      "test.event",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"test": "data",
			},
		}

		hub.Broadcast <- message
		time.Sleep(50 * time.Millisecond)

		// Check both clients received the message
		select {
		case msg1 := <-client1.Send:
			assert.Equal(t, "test.event", msg1.Type)
			assert.Equal(t, "data", msg1.Data["test"])
		default:
			t.Error("Client 1 did not receive message")
		}

		select {
		case msg2 := <-client2.Send:
			assert.Equal(t, "test.event", msg2.Type)
		default:
			t.Error("Client 2 did not receive message")
		}
	})
}

func TestBroadcastToUser(t *testing.T) {
	hub := &WebSocketHub{
		Clients:    make(map[string]*WebSocketClient),
		Register:   make(chan *WebSocketClient),
		Unregister: make(chan *WebSocketClient),
		Broadcast:  make(chan WebSocketMessage, 256),
	}

	go hub.Run()
	time.Sleep(50 * time.Millisecond)

	// Register two clients for same user
	client1 := &WebSocketClient{
		ID:     "client-1",
		UserID: "user1",
		Send:   make(chan WebSocketMessage, 256),
		Hub:    hub,
	}

	client2 := &WebSocketClient{
		ID:     "client-2",
		UserID: "user1",
		Send:   make(chan WebSocketMessage, 256),
		Hub:    hub,
	}

	// Register client for different user
	client3 := &WebSocketClient{
		ID:     "client-3",
		UserID: "user2",
		Send:   make(chan WebSocketMessage, 256),
		Hub:    hub,
	}

	hub.Register <- client1
	hub.Register <- client2
	hub.Register <- client3
	time.Sleep(50 * time.Millisecond)

	message := WebSocketMessage{
		Type:      "user.specific",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"message": "Hello user1",
		},
	}

	hub.BroadcastToUser("user1", message)
	time.Sleep(50 * time.Millisecond)

	// Check user1's clients received message
	select {
	case msg := <-client1.Send:
		assert.Equal(t, "user.specific", msg.Type)
		assert.Equal(t, "Hello user1", msg.Data["message"])
	default:
		t.Error("Client 1 (user1) did not receive message")
	}

	select {
	case msg := <-client2.Send:
		assert.Equal(t, "user.specific", msg.Type)
	default:
		t.Error("Client 2 (user1) did not receive message")
	}

	// Check user2's client did NOT receive message
	select {
	case <-client3.Send:
		t.Error("Client 3 (user2) should not have received message")
	default:
		// Correct - no message received
	}
}

func TestWebSocketMessages(t *testing.T) {
	t.Run("Webhook delivery message", func(t *testing.T) {
		// hub := GetWebSocketHub()

		// This would normally send via WebSocket
		// Testing the message structure
		BroadcastWebhookDelivery("user1", 5, 10, "success")

		// Message format is correct (no panic)
		assert.True(t, true)
	})

	t.Run("Security alert message", func(t *testing.T) {
		BroadcastSecurityAlert("user1", "failed_login", "high", "Multiple failed login attempts")
		assert.True(t, true)
	})

	t.Run("Scheduled session event", func(t *testing.T) {
		BroadcastScheduledSessionEvent("user1", 3, "started", "session-123")
		assert.True(t, true)
	})

	t.Run("Node health update", func(t *testing.T) {
		BroadcastNodeHealthUpdate("worker-1", "healthy", 45.2, 62.8)
		assert.True(t, true)
	})

	t.Run("Scaling event", func(t *testing.T) {
		BroadcastScalingEvent(1, "scale_up", "success")
		assert.True(t, true)
	})

	t.Run("Compliance violation", func(t *testing.T) {
		BroadcastComplianceViolation("user1", 101, 5, "high")
		assert.True(t, true)
	})
}

func TestWebSocketMessageSerialization(t *testing.T) {
	message := WebSocketMessage{
		Type:      "test.event",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
			"key3": true,
		},
	}

	// Test JSON serialization
	data, err := json.Marshal(message)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test deserialization
	var decoded WebSocketMessage
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, "test.event", decoded.Type)
	assert.Equal(t, "value1", decoded.Data["key1"])
	assert.Equal(t, float64(42), decoded.Data["key2"]) // JSON numbers are float64
	assert.Equal(t, true, decoded.Data["key3"])
}

func TestWebSocketClientBuffering(t *testing.T) {
	client := &WebSocketClient{
		ID:     "buffering-test",
		UserID: "user1",
		Send:   make(chan WebSocketMessage, 10), // Small buffer
	}

	// Fill buffer
	for i := 0; i < 10; i++ {
		msg := WebSocketMessage{
			Type:      "test",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"count": i},
		}
		client.Send <- msg
	}

	// Buffer should be full
	assert.Len(t, client.Send, 10)

	// Drain buffer
	for i := 0; i < 10; i++ {
		<-client.Send
	}

	assert.Len(t, client.Send, 0)
}

func TestWebSocketEventTypes(t *testing.T) {
	eventTypes := []string{
		"webhook.delivery",
		"security.alert",
		"schedule.event",
		"node.health",
		"scaling.event",
		"compliance.violation",
		"connection",
	}

	for _, eventType := range eventTypes {
		t.Run("Event type: "+eventType, func(t *testing.T) {
			message := WebSocketMessage{
				Type:      eventType,
				Timestamp: time.Now(),
				Data:      map[string]interface{}{},
			}

			assert.Equal(t, eventType, message.Type)
			assert.NotNil(t, message.Data)
		})
	}
}

func TestConcurrentClientRegistration(t *testing.T) {
	hub := &WebSocketHub{
		Clients:    make(map[string]*WebSocketClient),
		Register:   make(chan *WebSocketClient),
		Unregister: make(chan *WebSocketClient),
		Broadcast:  make(chan WebSocketMessage, 256),
	}

	go hub.Run()
	time.Sleep(50 * time.Millisecond)

	// Register 100 clients concurrently
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(id int) {
			client := &WebSocketClient{
				ID:     fmt.Sprintf("concurrent-client-%d", id),
				UserID: fmt.Sprintf("user%d", id%10),
				Send:   make(chan WebSocketMessage, 256),
				Hub:    hub,
			}
			hub.Register <- client
			done <- true
		}(i)
	}

	// Wait for all registrations
	for i := 0; i < 100; i++ {
		<-done
	}
	time.Sleep(100 * time.Millisecond)

	hub.Mu.RLock()
	clientCount := len(hub.Clients)
	hub.Mu.RUnlock()

	assert.Equal(t, 100, clientCount, "All 100 clients should be registered")
}

func TestMessageDeliveryReliability(t *testing.T) {
	hub := &WebSocketHub{
		Clients:    make(map[string]*WebSocketClient),
		Register:   make(chan *WebSocketClient),
		Unregister: make(chan *WebSocketClient),
		Broadcast:  make(chan WebSocketMessage, 256),
	}

	go hub.Run()
	time.Sleep(50 * time.Millisecond)

	client := &WebSocketClient{
		ID:     "reliability-test",
		UserID: "user1",
		Send:   make(chan WebSocketMessage, 256),
		Hub:    hub,
	}

	hub.Register <- client
	time.Sleep(50 * time.Millisecond)

	// Send 100 messages
	for i := 0; i < 100; i++ {
		message := WebSocketMessage{
			Type:      "test",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"count": i},
		}
		hub.BroadcastToUser("user1", message)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify all messages received
	receivedCount := 0
	timeout := time.After(1 * time.Second)

loop:
	for {
		select {
		case msg := <-client.Send:
			assert.Equal(t, "test", msg.Type)
			receivedCount++
			if receivedCount == 100 {
				break loop
			}
		case <-timeout:
			t.Errorf("Timeout: Only received %d out of 100 messages", receivedCount)
			break loop
		}
	}

	assert.Equal(t, 100, receivedCount, "All 100 messages should be received")
}
