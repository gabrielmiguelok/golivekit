// Package main demonstrates a real-time chat using GoliveKit.
package main

import (
	"context"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gabrielmiguelok/golivekit/client"
	"github.com/gabrielmiguelok/golivekit/pkg/core"
	"github.com/gabrielmiguelok/golivekit/pkg/pubsub"
	"github.com/gabrielmiguelok/golivekit/pkg/router"
)

// Global message store (in production, use a database)
var messageStore = &MessageStore{
	messages: make([]Message, 0),
}

// Global PubSub for broadcasting messages
var ps = pubsub.NewMemoryPubSub()

func main() {
	// Create router
	r := router.New()

	// Set custom PubSub
	r.SetPubSub(ps)

	// Serve GoliveKit client JS
	r.Handle("/_live/", http.StripPrefix("/_live/", client.Handler()))

	// Register LiveView route
	r.Live("/", NewChatRoom)

	log.Println("💬 Chat example starting at http://localhost:3000")
	log.Println("Open multiple browser tabs to test real-time chat!")
	log.Println("Press Ctrl+C to stop")
	log.Fatal(http.ListenAndServe(":3000", r))
}

// Message represents a chat message.
type Message struct {
	ID        string
	Username  string
	Content   string
	Timestamp time.Time
}

// MessageStore stores messages thread-safely.
type MessageStore struct {
	messages []Message
	mu       sync.RWMutex
}

// Add adds a message to the store.
func (ms *MessageStore) Add(msg Message) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.messages = append(ms.messages, msg)

	// Keep only last 100 messages
	if len(ms.messages) > 100 {
		ms.messages = ms.messages[len(ms.messages)-100:]
	}
}

// All returns all messages.
func (ms *MessageStore) All() []Message {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	result := make([]Message, len(ms.messages))
	copy(result, ms.messages)
	return result
}

// ChatRoom is the chat room LiveView component.
type ChatRoom struct {
	core.BaseComponent
	Username string
	sub      pubsub.Subscription
}

// NewChatRoom creates a new chat room component.
func NewChatRoom() core.Component {
	return &ChatRoom{}
}

// Name returns the component name.
func (c *ChatRoom) Name() string {
	return "chat"
}

// Mount initializes the chat room.
func (c *ChatRoom) Mount(ctx context.Context, params core.Params, session core.Session) error {
	// Generate a random username
	c.Username = fmt.Sprintf("User%d", time.Now().UnixNano()%10000)

	// Allow custom username from query params
	if username := params.Get("username"); username != "" {
		c.Username = username
	}

	c.Assigns().Set("username", c.Username)
	c.Assigns().Set("messages", messageStore.All())

	// Subscribe to new messages
	var err error
	c.sub, err = ps.Subscribe("chat:messages", func(data []byte) {
		// Trigger a re-render when new messages arrive
		// This will be handled by the LiveView system
		c.Assigns().Set("messages", messageStore.All())
		// Note: In a full implementation, this would trigger a push to the client
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	return nil
}

// HandleEvent handles user interactions.
func (c *ChatRoom) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	switch event {
	case "send_message":
		content, _ := payload["message"].(string)
		if content == "" {
			return nil
		}

		// Create message
		msg := Message{
			ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
			Username:  c.Username,
			Content:   content,
			Timestamp: time.Now(),
		}

		// Store message
		messageStore.Add(msg)

		// Broadcast to all subscribers
		ps.Publish("chat:messages", []byte(msg.ID))

		// Update local assigns
		c.Assigns().Set("messages", messageStore.All())

	case "set_username":
		username, _ := payload["username"].(string)
		if username != "" {
			c.Username = username
			c.Assigns().Set("username", c.Username)
		}
	}

	return nil
}

// Terminate cleans up resources.
func (c *ChatRoom) Terminate(ctx context.Context, reason core.TerminateReason) error {
	if c.sub != nil {
		c.sub.Unsubscribe()
	}
	return nil
}

// Render returns the HTML representation.
func (c *ChatRoom) Render(ctx context.Context) core.Renderer {
	return core.RendererFunc(func(ctx context.Context, w io.Writer) error {
		messages := messageStore.All()

		// Build messages HTML
		var messagesHTML string
		if len(messages) == 0 {
			messagesHTML = `<p class="no-messages">No messages yet. Be the first to say hello!</p>`
		} else {
			for _, msg := range messages {
				isOwn := msg.Username == c.Username
				class := "message"
				if isOwn {
					class += " own"
				}
				messagesHTML += fmt.Sprintf(`
                <div class="%s">
                    <div class="message-header">
                        <span class="username">%s</span>
                        <span class="time">%s</span>
                    </div>
                    <div class="content">%s</div>
                </div>`,
					class,
					html.EscapeString(msg.Username),
					msg.Timestamp.Format("15:04:05"),
					html.EscapeString(msg.Content),
				)
			}
		}

		tmpl := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>GoliveKit Chat Example</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 1rem;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
        }
        .container {
            max-width: 800px;
            margin: 0 auto;
        }
        h1 {
            color: white;
            text-align: center;
            margin-bottom: 1rem;
            text-shadow: 0 2px 4px rgba(0,0,0,0.2);
        }
        .chat-box {
            background: white;
            border-radius: 1rem;
            box-shadow: 0 10px 40px rgba(0,0,0,0.2);
            overflow: hidden;
        }
        .user-info {
            padding: 1rem;
            background: #f3f4f6;
            border-bottom: 1px solid #e5e7eb;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        .user-info label {
            font-weight: 500;
            color: #374151;
        }
        .user-info input {
            flex: 1;
            padding: 0.5rem;
            border: 1px solid #d1d5db;
            border-radius: 0.375rem;
            font-size: 0.875rem;
        }
        .messages {
            height: 400px;
            overflow-y: auto;
            padding: 1rem;
            background: #fafafa;
        }
        .no-messages {
            text-align: center;
            color: #9ca3af;
            padding: 2rem;
        }
        .message {
            margin-bottom: 0.75rem;
            padding: 0.75rem;
            background: white;
            border-radius: 0.5rem;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
            max-width: 80%%;
        }
        .message.own {
            background: #dbeafe;
            margin-left: auto;
        }
        .message-header {
            display: flex;
            justify-content: space-between;
            margin-bottom: 0.25rem;
            font-size: 0.75rem;
        }
        .username {
            font-weight: 600;
            color: #4f46e5;
        }
        .message.own .username {
            color: #1e40af;
        }
        .time {
            color: #9ca3af;
        }
        .content {
            color: #374151;
            word-wrap: break-word;
        }
        .input-area {
            padding: 1rem;
            border-top: 1px solid #e5e7eb;
            display: flex;
            gap: 0.5rem;
        }
        .input-area input {
            flex: 1;
            padding: 0.75rem;
            border: 1px solid #d1d5db;
            border-radius: 0.5rem;
            font-size: 1rem;
        }
        .input-area input:focus {
            outline: none;
            border-color: #4f46e5;
            box-shadow: 0 0 0 3px rgba(79, 70, 229, 0.1);
        }
        .input-area button {
            padding: 0.75rem 1.5rem;
            background: #4f46e5;
            color: white;
            border: none;
            border-radius: 0.5rem;
            font-size: 1rem;
            cursor: pointer;
            transition: background 0.15s;
        }
        .input-area button:hover {
            background: #4338ca;
        }
    </style>
</head>
<body>
    <div data-live-view="chat">
        <div class="container">
            <h1>💬 GoliveKit Chat</h1>
            <div class="chat-box">
                <div class="user-info">
                    <label>Your name:</label>
                    <input type="text" value="%s" lv-change="set_username" lv-debounce="500" name="username" />
                </div>
                <div class="messages" data-slot="messages">
                    %s
                </div>
                <form class="input-area" lv-submit="send_message">
                    <input type="text" name="message" placeholder="Type a message..." autocomplete="off" />
                    <button type="submit">Send</button>
                </form>
            </div>
        </div>
    </div>
    <script src="/_live/golivekit.js"></script>
</body>
</html>`,
			html.EscapeString(c.Username),
			messagesHTML,
		)

		_, err := w.Write([]byte(tmpl))
		return err
	})
}
