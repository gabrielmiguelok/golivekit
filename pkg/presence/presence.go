// Package presence provides user presence tracking for GoliveKit.
package presence

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gabrielmiguelok/golivekit/pkg/core"
	"github.com/gabrielmiguelok/golivekit/pkg/pubsub"
)

// Presence tracks user presence in a topic.
type Presence struct {
	topic     string
	presences map[string]*PresenceInfo
	pubsub    pubsub.PubSub
	onJoin    []func(PresenceInfo)
	onLeave   []func(PresenceInfo)
	mu        sync.RWMutex
}

// PresenceInfo contains information about a present user.
type PresenceInfo struct {
	// Key is the unique identifier (usually socket ID).
	Key string `json:"key"`

	// UserID is the user's identifier.
	UserID string `json:"user_id"`

	// Username is the user's display name.
	Username string `json:"username"`

	// OnlineAt is when the user came online.
	OnlineAt time.Time `json:"online_at"`

	// PhxRef is the Phoenix reference (for compatibility).
	PhxRef string `json:"phx_ref,omitempty"`

	// Metas contains additional metadata.
	Metas map[string]any `json:"metas,omitempty"`
}

// NewPresence creates a new presence tracker for a topic.
func NewPresence(topic string, ps pubsub.PubSub) *Presence {
	p := &Presence{
		topic:     topic,
		presences: make(map[string]*PresenceInfo),
		pubsub:    ps,
		onJoin:    make([]func(PresenceInfo), 0),
		onLeave:   make([]func(PresenceInfo), 0),
	}

	// Subscribe to presence events
	if ps != nil {
		ps.Subscribe(topic+":presence", func(msg []byte) {
			p.handlePresenceMessage(msg)
		})
	}

	return p
}

// Track adds a socket to presence tracking.
func (p *Presence) Track(socket *core.Socket, info PresenceInfo) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	info.Key = socket.ID()
	info.OnlineAt = time.Now()

	p.presences[socket.ID()] = &info

	// Notify local handlers
	for _, handler := range p.onJoin {
		go handler(info)
	}

	// Broadcast to other nodes
	if p.pubsub != nil {
		msg := presenceMessage{
			Type: "join",
			Info: info,
		}
		data, _ := json.Marshal(msg)
		p.pubsub.Publish(p.topic+":presence", data)
	}

	return nil
}

// Untrack removes a socket from presence tracking.
func (p *Presence) Untrack(socket *core.Socket) error {
	p.mu.Lock()
	info, exists := p.presences[socket.ID()]
	if exists {
		delete(p.presences, socket.ID())
	}
	p.mu.Unlock()

	if !exists {
		return nil
	}

	// Notify local handlers
	for _, handler := range p.onLeave {
		go handler(*info)
	}

	// Broadcast to other nodes
	if p.pubsub != nil {
		msg := presenceMessage{
			Type: "leave",
			Info: *info,
		}
		data, _ := json.Marshal(msg)
		p.pubsub.Publish(p.topic+":presence", data)
	}

	return nil
}

// Update updates presence metadata for a socket.
func (p *Presence) Update(socket *core.Socket, metas map[string]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	info, exists := p.presences[socket.ID()]
	if !exists {
		return nil
	}

	if info.Metas == nil {
		info.Metas = make(map[string]any)
	}
	for k, v := range metas {
		info.Metas[k] = v
	}

	return nil
}

// List returns all currently present users.
func (p *Presence) List() []PresenceInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]PresenceInfo, 0, len(p.presences))
	for _, info := range p.presences {
		result = append(result, *info)
	}
	return result
}

// Get retrieves presence info by key.
func (p *Presence) Get(key string) (*PresenceInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	info, ok := p.presences[key]
	if !ok {
		return nil, false
	}
	return info, true
}

// Count returns the number of present users.
func (p *Presence) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.presences)
}

// OnJoin registers a handler for join events.
func (p *Presence) OnJoin(handler func(PresenceInfo)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onJoin = append(p.onJoin, handler)
}

// OnLeave registers a handler for leave events.
func (p *Presence) OnLeave(handler func(PresenceInfo)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onLeave = append(p.onLeave, handler)
}

// Diff returns the difference between two presence states.
func (p *Presence) Diff(previous []PresenceInfo) PresenceDiff {
	current := p.List()

	diff := PresenceDiff{
		Joins:  make([]PresenceInfo, 0),
		Leaves: make([]PresenceInfo, 0),
	}

	// Build lookup maps
	prevMap := make(map[string]bool)
	for _, info := range previous {
		prevMap[info.Key] = true
	}

	currMap := make(map[string]bool)
	for _, info := range current {
		currMap[info.Key] = true
	}

	// Find joins
	for _, info := range current {
		if !prevMap[info.Key] {
			diff.Joins = append(diff.Joins, info)
		}
	}

	// Find leaves
	for _, info := range previous {
		if !currMap[info.Key] {
			diff.Leaves = append(diff.Leaves, info)
		}
	}

	return diff
}

// PresenceDiff represents changes in presence state.
type PresenceDiff struct {
	Joins  []PresenceInfo `json:"joins"`
	Leaves []PresenceInfo `json:"leaves"`
}

// IsEmpty returns true if there are no changes.
func (d PresenceDiff) IsEmpty() bool {
	return len(d.Joins) == 0 && len(d.Leaves) == 0
}

// ToMap converts the diff to a map format (for Phoenix compatibility).
func (d PresenceDiff) ToMap() map[string]any {
	joinsMap := make(map[string]any)
	for _, info := range d.Joins {
		joinsMap[info.Key] = map[string]any{
			"metas": []map[string]any{
				{
					"user_id":   info.UserID,
					"username":  info.Username,
					"online_at": info.OnlineAt.Unix(),
					"phx_ref":   info.PhxRef,
				},
			},
		}
	}

	leavesMap := make(map[string]any)
	for _, info := range d.Leaves {
		leavesMap[info.Key] = map[string]any{
			"metas": []map[string]any{
				{
					"user_id":   info.UserID,
					"username":  info.Username,
					"online_at": info.OnlineAt.Unix(),
					"phx_ref":   info.PhxRef,
				},
			},
		}
	}

	return map[string]any{
		"joins":  joinsMap,
		"leaves": leavesMap,
	}
}

type presenceMessage struct {
	Type string       `json:"type"`
	Info PresenceInfo `json:"info"`
}

func (p *Presence) handlePresenceMessage(data []byte) {
	var msg presenceMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "join":
		p.mu.Lock()
		p.presences[msg.Info.Key] = &msg.Info
		p.mu.Unlock()

		for _, handler := range p.onJoin {
			go handler(msg.Info)
		}

	case "leave":
		p.mu.Lock()
		delete(p.presences, msg.Info.Key)
		p.mu.Unlock()

		for _, handler := range p.onLeave {
			go handler(msg.Info)
		}
	}
}

// PresenceManager manages presence across multiple topics.
type PresenceManager struct {
	presences map[string]*Presence
	pubsub    pubsub.PubSub
	mu        sync.RWMutex
}

// NewPresenceManager creates a new presence manager.
func NewPresenceManager(ps pubsub.PubSub) *PresenceManager {
	return &PresenceManager{
		presences: make(map[string]*Presence),
		pubsub:    ps,
	}
}

// GetOrCreate gets or creates a presence tracker for a topic.
func (pm *PresenceManager) GetOrCreate(topic string) *Presence {
	pm.mu.RLock()
	p, exists := pm.presences[topic]
	pm.mu.RUnlock()

	if exists {
		return p
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Double-check after acquiring write lock
	if p, exists = pm.presences[topic]; exists {
		return p
	}

	p = NewPresence(topic, pm.pubsub)
	pm.presences[topic] = p
	return p
}

// Remove removes a presence tracker.
func (pm *PresenceManager) Remove(topic string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.presences, topic)
}

// Topics returns all tracked topics.
func (pm *PresenceManager) Topics() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	topics := make([]string, 0, len(pm.presences))
	for topic := range pm.presences {
		topics = append(topics, topic)
	}
	return topics
}
