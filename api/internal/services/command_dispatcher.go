// Package services provides business logic services for the StreamSpace API.
// This file implements the CommandDispatcher for queuing and dispatching commands to agents.
//
// COMMAND DISPATCHER:
// The CommandDispatcher is responsible for:
//   - Queuing commands for dispatch to agents
//   - Managing a worker pool to process commands concurrently
//   - Sending commands to agents via the AgentHub
//   - Updating command status in the database
//   - Handling command lifecycle (pending → sent → ack → completed/failed)
//
// COMMAND LIFECYCLE:
//  1. Command created in database with status="pending"
//  2. DispatchCommand() queues the command
//  3. Worker picks up command from queue
//  4. Worker checks if agent is connected
//  5. Worker sends command to agent via hub
//  6. Worker updates status="sent" and sent_at timestamp
//  7. Agent acknowledges (WebSocket handler updates status="ack")
//  8. Agent completes/fails (WebSocket handler updates status="completed"/"failed")
//
// WORKER POOL PATTERN:
// The dispatcher uses a worker pool to process commands concurrently.
// Each worker is a goroutine that continuously reads from the queue channel.
//
// Example:
//
//	dispatcher := NewCommandDispatcher(database, hub)
//	go dispatcher.Start()
//
//	command := &models.AgentCommand{
//	    CommandID: "cmd-123",
//	    AgentID: "k8s-prod-us-east-1",
//	    Action: "start_session",
//	    Payload: &models.CommandPayload{"sessionId": "sess-456"},
//	    Status: "pending",
//	}
//	err := dispatcher.DispatchCommand(command)
package services

import (
	"fmt"
	"log"
	"time"

	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/models"
	"github.com/streamspace-dev/streamspace/api/internal/websocket"
)

// CommandDispatcher manages the queuing and dispatch of commands to agents.
//
// The dispatcher maintains a worker pool that continuously processes commands
// from the queue channel. Each worker checks if the target agent is connected
// and sends the command via the AgentHub.
type CommandDispatcher struct {
	// database is used to update command status
	database *db.Database

	// hub is used to send commands to agents
	hub *websocket.AgentHub

	// queue is the channel for pending commands
	queue chan *models.AgentCommand

	// workers is the number of worker goroutines
	workers int

	// stopChan signals workers to stop
	stopChan chan struct{}
}

// NewCommandDispatcher creates a new CommandDispatcher.
//
// The dispatcher is initialized with a buffered queue channel and configured
// number of workers (default: 10).
//
// Example:
//
//	dispatcher := NewCommandDispatcher(database, hub)
//	go dispatcher.Start()
func NewCommandDispatcher(database *db.Database, hub *websocket.AgentHub) *CommandDispatcher {
	return &CommandDispatcher{
		database: database,
		hub:      hub,
		queue:    make(chan *models.AgentCommand, 1000),
		workers:  10, // Default 10 workers
		stopChan: make(chan struct{}),
	}
}

// SetWorkers configures the number of worker goroutines.
//
// This should be called before Start().
//
// Example:
//
//	dispatcher.SetWorkers(20)
//	go dispatcher.Start()
func (d *CommandDispatcher) SetWorkers(count int) {
	if count > 0 {
		d.workers = count
	}
}

// Start starts the worker pool.
//
// This function starts the configured number of worker goroutines.
// Each worker continuously processes commands from the queue channel.
//
// This function blocks until Stop() is called.
//
// Example:
//
//	dispatcher := NewCommandDispatcher(database, hub)
//	go dispatcher.Start()
func (d *CommandDispatcher) Start() {
	log.Printf("[CommandDispatcher] Starting with %d workers", d.workers)

	// Start worker goroutines
	for i := 0; i < d.workers; i++ {
		go d.worker(i)
	}

	// Wait for stop signal
	<-d.stopChan
	log.Println("[CommandDispatcher] Stopped")
}

// Stop signals the dispatcher to stop.
//
// This closes the stopChan, causing Start() to exit.
// Workers will finish processing their current commands before exiting.
func (d *CommandDispatcher) Stop() {
	close(d.stopChan)
}

// DispatchCommand queues a command for dispatch to an agent.
//
// The command should already be created in the database with status="pending".
// This function adds the command to the queue for processing by a worker.
//
// Returns an error if the queue is full.
//
// Example:
//
//	command := &models.AgentCommand{
//	    CommandID: "cmd-123",
//	    AgentID: "k8s-prod-us-east-1",
//	    Action: "start_session",
//	    Payload: &models.CommandPayload{"sessionId": "sess-456"},
//	    Status: "pending",
//	}
//	err := dispatcher.DispatchCommand(command)
func (d *CommandDispatcher) DispatchCommand(command *models.AgentCommand) error {
	if command == nil {
		return fmt.Errorf("command cannot be nil")
	}

	if command.CommandID == "" {
		return fmt.Errorf("command_id cannot be empty")
	}

	if command.AgentID == "" {
		return fmt.Errorf("agent_id cannot be empty")
	}

	select {
	case d.queue <- command:
		log.Printf("[CommandDispatcher] Queued command %s for agent %s (action: %s)",
			command.CommandID, command.AgentID, command.Action)
		return nil
	default:
		return fmt.Errorf("command queue is full")
	}
}

// worker is a worker goroutine that processes commands from the queue.
//
// Each worker continuously reads from the queue channel and dispatches
// commands to agents via the AgentHub.
//
// Workers run until the stopChan is closed.
func (d *CommandDispatcher) worker(workerID int) {
	log.Printf("[CommandDispatcher] Worker %d started", workerID)

	for {
		select {
		case command := <-d.queue:
			d.processCommand(workerID, command)

		case <-d.stopChan:
			log.Printf("[CommandDispatcher] Worker %d stopped", workerID)
			return
		}
	}
}

// processCommand processes a single command.
//
// Flow:
//  1. Check if agent is connected
//  2. Send command to agent via hub
//  3. Update command status to "sent" in database
//  4. Handle errors (update status to "failed")
func (d *CommandDispatcher) processCommand(workerID int, command *models.AgentCommand) {
	log.Printf("[CommandDispatcher] Worker %d processing command %s for agent %s",
		workerID, command.CommandID, command.AgentID)

	// Check if agent is connected
	if !d.hub.IsAgentConnected(command.AgentID) {
		log.Printf("[CommandDispatcher] Agent %s is not connected, marking command %s as failed",
			command.AgentID, command.CommandID)
		d.failCommand(command, "agent is not connected")
		return
	}

	// Send command to agent
	if err := d.sendToAgent(command); err != nil {
		log.Printf("[CommandDispatcher] Failed to send command %s to agent %s: %v",
			command.CommandID, command.AgentID, err)
		d.failCommand(command, err.Error())
		return
	}

	// Update command status to "sent"
	now := time.Now()
	_, err := d.database.DB().Exec(`
		UPDATE agent_commands
		SET status = 'sent', sent_at = $1, updated_at = $1
		WHERE command_id = $2
	`, now, command.CommandID)

	if err != nil {
		log.Printf("[CommandDispatcher] Failed to update command %s status to sent: %v",
			command.CommandID, err)
		// Don't fail the command here - it was sent successfully
		// The status update failure is a database issue, not a command failure
		return
	}

	log.Printf("[CommandDispatcher] Worker %d sent command %s to agent %s",
		workerID, command.CommandID, command.AgentID)
}

// sendToAgent sends a command to an agent via the AgentHub.
//
// Returns an error if the send fails (agent disconnected, buffer full, etc.).
func (d *CommandDispatcher) sendToAgent(command *models.AgentCommand) error {
	return d.hub.SendCommandToAgent(command.AgentID, command)
}

// failCommand updates a command's status to "failed" in the database.
//
// This is called when:
//   - Agent is not connected
//   - Send to agent fails
//   - Other dispatch errors occur
func (d *CommandDispatcher) failCommand(command *models.AgentCommand, errorMessage string) {
	now := time.Now()
	_, err := d.database.DB().Exec(`
		UPDATE agent_commands
		SET status = 'failed', error_message = $1, updated_at = $2
		WHERE command_id = $3
	`, errorMessage, now, command.CommandID)

	if err != nil {
		log.Printf("[CommandDispatcher] Failed to update command %s status to failed: %v",
			command.CommandID, err)
	}
}

// GetQueueLength returns the current number of commands in the queue.
//
// Useful for monitoring and debugging.
//
// Example:
//
//	length := dispatcher.GetQueueLength()
//	fmt.Printf("Commands in queue: %d\n", length)
func (d *CommandDispatcher) GetQueueLength() int {
	return len(d.queue)
}

// GetQueueCapacity returns the maximum capacity of the command queue.
//
// Useful for monitoring and debugging.
//
// Example:
//
//	capacity := dispatcher.GetQueueCapacity()
//	fmt.Printf("Queue capacity: %d\n", capacity)
func (d *CommandDispatcher) GetQueueCapacity() int {
	return cap(d.queue)
}

// DispatchPendingCommands retrieves all pending commands from the database
// and queues them for dispatch.
//
// This is useful for recovering from a Control Plane restart - any commands
// that were pending when the server stopped will be re-queued.
//
// Example:
//
//	// On server startup:
//	dispatcher := NewCommandDispatcher(database, hub)
//	go dispatcher.Start()
//	dispatcher.DispatchPendingCommands()
func (d *CommandDispatcher) DispatchPendingCommands() error {
	rows, err := d.database.DB().Query(`
		SELECT id, command_id, agent_id, session_id, action, payload, status, error_message, created_at, sent_at, acknowledged_at, completed_at
		FROM agent_commands
		WHERE status = 'pending'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return fmt.Errorf("failed to query pending commands: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var command models.AgentCommand
		err := rows.Scan(
			&command.ID,
			&command.CommandID,
			&command.AgentID,
			&command.SessionID,
			&command.Action,
			&command.Payload,
			&command.Status,
			&command.ErrorMessage,
			&command.CreatedAt,
			&command.SentAt,
			&command.AcknowledgedAt,
			&command.CompletedAt,
		)
		if err != nil {
			log.Printf("[CommandDispatcher] Failed to scan pending command: %v", err)
			continue
		}

		if err := d.DispatchCommand(&command); err != nil {
			log.Printf("[CommandDispatcher] Failed to queue pending command %s: %v", command.CommandID, err)
			continue
		}

		count++
	}

	if count > 0 {
		log.Printf("[CommandDispatcher] Queued %d pending commands for dispatch", count)
	}

	return nil
}
