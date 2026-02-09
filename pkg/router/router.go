// Package router provides HTTP routing for GoliveKit applications.
package router

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"strings"
	"sync"

	"github.com/gabrielmiguelok/golivekit/pkg/core"
	"github.com/gabrielmiguelok/golivekit/pkg/diff"
	"github.com/gabrielmiguelok/golivekit/pkg/pool"
	"github.com/gabrielmiguelok/golivekit/pkg/protocol"
	"github.com/gabrielmiguelok/golivekit/pkg/pubsub"
	"github.com/gabrielmiguelok/golivekit/pkg/security"
	"github.com/gabrielmiguelok/golivekit/pkg/transport"
)

// Common router errors.
var (
	ErrComponentNotFound  = errors.New("component not found")
	ErrSessionNotFound    = errors.New("session not found")
	ErrWebSocketRequired  = errors.New("websocket connection required")
)

// Router handles HTTP routing for GoliveKit.
type Router struct {
	mux          *http.ServeMux
	liveRoutes   map[string]*LiveRoute
	middleware   []Middleware
	errorHandler ErrorHandler
	notFound     http.Handler
	registry     *core.ComponentRegistry

	// Session and connection management
	sessionManager *LiveViewSessionManager
	socketManager  *core.SocketManager

	// Protocol handling
	codec protocol.Codec

	// Diff engine for computing HTML diffs
	diffEngine *diff.Engine

	// PubSub for real-time messaging
	pubsub pubsub.PubSub

	mu sync.RWMutex
}

// LiveRoute defines a route that renders a LiveView component.
type LiveRoute struct {
	// Path is the URL path pattern.
	Path string

	// Component is the factory function for creating the component.
	Component func() core.Component

	// Layout is an optional layout component.
	Layout func() core.Component

	// Hooks are plugin hooks to run for this route.
	Hooks []string

	// Middleware are route-specific middleware.
	Middleware []Middleware

	// Meta contains route metadata.
	Meta map[string]any
}

// Middleware is a function that wraps an HTTP handler.
type Middleware func(http.Handler) http.Handler

// ErrorHandler handles errors during request processing.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

// New creates a new router.
func New() *Router {
	return &Router{
		mux:        http.NewServeMux(),
		liveRoutes: make(map[string]*LiveRoute),
		middleware: make([]Middleware, 0),
		registry:   core.NewComponentRegistry(),

		// Initialize E2E components
		sessionManager: NewLiveViewSessionManager(),
		socketManager:  core.NewSocketManager(),
		codec:          protocol.NewPhoenixCodec(),
		diffEngine:     diff.NewEngine(),
		pubsub:         pubsub.NewMemoryPubSub(),

		errorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		},
		notFound: http.NotFoundHandler(),
	}
}

// Use adds middleware to the router.
func (r *Router) Use(mw Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middleware = append(r.middleware, mw)
}

// SetErrorHandler sets the error handler.
func (r *Router) SetErrorHandler(handler ErrorHandler) {
	r.errorHandler = handler
}

// SetNotFoundHandler sets the 404 handler.
func (r *Router) SetNotFoundHandler(handler http.Handler) {
	r.notFound = handler
}

// Registry returns the component registry.
func (r *Router) Registry() *core.ComponentRegistry {
	return r.registry
}

// SessionManager returns the session manager.
func (r *Router) SessionManager() *LiveViewSessionManager {
	return r.sessionManager
}

// SocketManager returns the socket manager.
func (r *Router) SocketManager() *core.SocketManager {
	return r.socketManager
}

// PubSub returns the pubsub instance.
func (r *Router) PubSub() pubsub.PubSub {
	return r.pubsub
}

// SetPubSub sets a custom pubsub implementation.
func (r *Router) SetPubSub(ps pubsub.PubSub) {
	r.pubsub = ps
}

// Live registers a LiveView route.
func (r *Router) Live(path string, component func() core.Component, opts ...RouteOption) {
	route := &LiveRoute{
		Path:       path,
		Component:  component,
		Middleware: make([]Middleware, 0),
		Meta:       make(map[string]any),
	}

	for _, opt := range opts {
		opt(route)
	}

	r.mu.Lock()
	r.liveRoutes[path] = route
	r.mu.Unlock()

	r.mux.HandleFunc(path, r.handleLive(route))
}

// Handle registers a standard HTTP handler.
func (r *Router) Handle(pattern string, handler http.Handler) {
	// Apply global middleware
	r.mu.RLock()
	middleware := make([]Middleware, len(r.middleware))
	copy(middleware, r.middleware)
	r.mu.RUnlock()

	h := handler
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}

	r.mux.Handle(pattern, h)
}

// HandleFunc registers a standard HTTP handler function.
func (r *Router) HandleFunc(pattern string, handler http.HandlerFunc) {
	r.Handle(pattern, handler)
}

// Static serves static files from a directory.
func (r *Router) Static(prefix, dir string) {
	fs := http.FileServer(http.Dir(dir))
	r.mux.Handle(prefix, http.StripPrefix(prefix, fs))
}

// Group creates a route group with shared prefix and middleware.
func (r *Router) Group(prefix string, fn func(*RouteGroup)) {
	group := &RouteGroup{
		router:     r,
		prefix:     prefix,
		middleware: make([]Middleware, 0),
	}
	fn(group)
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// handleLive creates the HTTP handler for a LiveView route.
func (r *Router) handleLive(route *LiveRoute) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		// Apply route-specific middleware
		var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			r.renderLive(w, req, route)
		})

		// Apply route middleware
		for i := len(route.Middleware) - 1; i >= 0; i-- {
			handler = route.Middleware[i](handler)
		}

		// Apply global middleware
		r.mu.RLock()
		middleware := make([]Middleware, len(r.middleware))
		copy(middleware, r.middleware)
		r.mu.RUnlock()

		for i := len(middleware) - 1; i >= 0; i-- {
			handler = middleware[i](handler)
		}

		handler.ServeHTTP(w, req.WithContext(ctx))
	}
}

// renderLive renders a LiveView component.
func (r *Router) renderLive(w http.ResponseWriter, req *http.Request, route *LiveRoute) {
	// If this is a WebSocket upgrade request, handle separately
	if isWebSocketRequest(req) {
		component := route.Component()
		r.handleWebSocket(w, req, component)
		return
	}

	// Create component instance for initial HTTP render
	component := route.Component()

	// Extract params from URL
	params := extractParams(req)

	// Get session data
	session := r.extractSession(req)

	// Create context
	ctx := req.Context()

	// Mount the component
	if err := component.Mount(ctx, params, session); err != nil {
		r.errorHandler(w, req, err)
		return
	}

	// Render the component
	renderer := component.Render(ctx)
	if renderer == nil {
		r.errorHandler(w, req, ErrNilRenderer)
		return
	}

	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Render HTML
	if err := renderer.Render(ctx, w); err != nil {
		r.errorHandler(w, req, err)
		return
	}
}

// handleWebSocket handles WebSocket upgrade for LiveView.
func (r *Router) handleWebSocket(w http.ResponseWriter, req *http.Request, component core.Component) {
	// 1. Create WebSocket transport
	wsTransport := transport.NewWebSocketTransport(transport.DefaultTransportConfig())

	// 2. Upgrade connection
	if err := wsTransport.Upgrade(w, req); err != nil {
		r.errorHandler(w, req, fmt.Errorf("websocket upgrade failed: %w", err))
		return
	}

	// 3. Generate socket ID
	socketID := generateSocketID()

	// 4. Create adapter and socket
	adapter := NewTransportAdapter(wsTransport, r.codec)
	socket := core.NewSocket(socketID, adapter)

	// 5. Extract session/params
	session := r.extractSession(req)
	params := extractParams(req)

	// 6. Wire component to socket if it supports it
	if bc, ok := component.(interface{ SetSocket(*core.Socket) }); ok {
		bc.SetSocket(socket)
	}

	// 7. Create LiveView session
	lvSession := r.sessionManager.Create(socketID, component, params, session)
	lvSession.Transport = wsTransport
	lvSession.Socket = socket
	lvSession.DiffEngine = r.diffEngine
	lvSession.Codec = r.codec

	// 8. Add socket to manager
	r.socketManager.Add(socket)

	// 9. Start message loop in a goroutine
	// NOTE: Use context.Background() instead of req.Context() because
	// the WebSocket connection outlives the HTTP request. req.Context()
	// is canceled when the HTTP handler returns, but the WebSocket
	// connection should stay alive.
	ctx := core.BuildContext(context.Background(), socket, component, session, params)
	go r.messageLoop(ctx, lvSession)

	// 10. Cleanup on disconnect
	go func() {
		<-wsTransport.CloseChan()
		r.handleDisconnect(lvSession)
	}()
}

// messageLoop processes incoming WebSocket messages.
func (r *Router) messageLoop(ctx context.Context, session *LiveViewSession) {
	recvCh := session.Transport.Receive()

	for {
		select {
		case msg, ok := <-recvCh:
			if !ok {
				// Channel closed, connection ended
				return
			}

			// Update activity
			session.UpdateActivity()

			// Handle message based on event
			switch msg.Event {
			case "heartbeat", "phx_heartbeat":
				r.handleHeartbeat(session, msg)

			case "phx_join":
				r.handleJoin(ctx, session, msg)

			case "phx_leave":
				r.handleLeave(session, msg)
				return

			default:
				// User event (click, change, submit, etc.)
				if err := r.dispatchEvent(ctx, session, msg); err != nil {
					r.sendError(session, msg.Ref, msg.Topic, err)
					continue
				}
				r.renderAndSendDiff(ctx, session)
			}

		case <-ctx.Done():
			return
		}
	}
}

// handleJoin handles the phx_join event.
func (r *Router) handleJoin(ctx context.Context, session *LiveViewSession, msg transport.Message) {
	if transport.DebugWebSocket {
		fmt.Printf("[ROUTER DEBUG] handleJoin called: topic=%s, ref=%s\n", msg.Topic, msg.Ref)
	}

	component := session.Component

	// Store join ref
	if joinRef, ok := msg.Payload["join_ref"].(string); ok {
		session.SetJoinRef(joinRef)
	}

	// Mount component if not already mounted
	if !session.IsMounted() {
		if err := component.Mount(ctx, session.Params, session.Session); err != nil {
			r.sendError(session, msg.Ref, msg.Topic, err)
			return
		}
		session.SetMounted(true)
	}

	// Initial render
	renderer := component.Render(ctx)
	if renderer == nil {
		r.sendError(session, msg.Ref, msg.Topic, ErrNilRenderer)
		return
	}

	// Use buffer from pool to reduce GC pressure
	buf := pool.GetBuffer()
	defer pool.PutBuffer(buf)

	if err := renderer.Render(ctx, buf); err != nil {
		r.sendError(session, msg.Ref, msg.Topic, err)
		return
	}

	// Send join reply with rendered HTML
	r.sendReply(session, msg.Ref, msg.Topic, map[string]any{
		"rendered": map[string]any{
			"s": []string{buf.String()},
		},
	})
}

// handleHeartbeat handles heartbeat messages.
func (r *Router) handleHeartbeat(session *LiveViewSession, msg transport.Message) {
	session.Socket.UpdateActivity()
	r.sendReply(session, msg.Ref, msg.Topic, nil)
}

// handleLeave handles the phx_leave event.
func (r *Router) handleLeave(session *LiveViewSession, msg transport.Message) {
	ctx := context.Background()
	session.Component.Terminate(ctx, core.TerminateNormal)
	r.handleDisconnect(session)
}

// dispatchEvent dispatches a user event to the component.
func (r *Router) dispatchEvent(ctx context.Context, session *LiveViewSession, msg transport.Message) error {
	event := msg.Event

	// Extract value from payload if present
	payload := msg.Payload
	if payload == nil {
		payload = make(map[string]any)
	}

	return session.Component.HandleEvent(ctx, event, payload)
}

// renderAndSendDiff renders the component and sends an optimized diff.
// Uses buffer pool to reduce GC pressure.
func (r *Router) renderAndSendDiff(ctx context.Context, session *LiveViewSession) {
	component := session.Component

	// 1. Try to get assigns and check for changes
	assigns := r.getAssigns(component)

	// Note: We don't skip based on tracker.HasChanges() because:
	// - Components may modify struct fields directly without using Assigns.Set()
	// - The actual diff will be computed by comparing rendered output
	// - If nothing changed, the diff will be empty and won't be sent

	// 2. Render the component
	renderer := component.Render(ctx)
	if renderer == nil {
		return
	}

	// Use buffer from pool to reduce GC pressure
	buf := pool.GetBuffer()
	defer pool.PutBuffer(buf)

	if err := renderer.Render(ctx, buf); err != nil {
		return
	}

	html := buf.String()

	// 4. Build optimized diff payload
	payload := r.buildDiffPayload(ctx, session, component, html, assigns)

	// 5. Send diff (only if there's something to send)
	if !payload.IsEmpty() {
		session.Socket.SendOptimizedDiff(payload)

		// 6. Reset change tracker after successful send
		if assigns != nil && assigns.Tracker().HasChanges() {
			assigns.Tracker().Reset()
		}
	}
}

// buildDiffPayload constructs the optimized diff payload.
// Uses hash-based comparison O(1) and per-socket state (no global lock contention).
func (r *Router) buildDiffPayload(ctx context.Context, session *LiveViewSession, component core.Component, html string, assigns *core.Assigns) *core.DiffPayload {
	// Get or increment version
	session.mu.Lock()
	session.Version++
	version := session.Version
	session.mu.Unlock()

	payload := &core.DiffPayload{
		Version:   version,
		Slots:     make(map[string]string),
		HTMLSlots: make(map[string]string),
	}

	// Extract slots from rendered HTML using optimized O(n) parser
	textSlots, htmlSlots := extractSlotsOptimized(html)

	// Get previous hashes from per-socket state (no global lock!)
	prevHashes := session.GetSlotHashes()

	newHashes := make(map[string]uint64, len(textSlots)+len(htmlSlots))

	// Compare with hash O(1) instead of string O(n)
	for id, content := range textSlots {
		hash := hashSlotContent(content)
		newHashes[id] = hash
		if prevHashes == nil || prevHashes[id] != hash {
			payload.Slots[id] = content
		}
	}

	for id, content := range htmlSlots {
		hash := hashSlotContent(content)
		newHashes[id] = hash
		if prevHashes == nil || prevHashes[id] != hash {
			payload.HTMLSlots[id] = content
		}
	}

	// Store new hashes in per-socket state (no global lock!)
	session.SetSlotHashes(newHashes)

	// If no slots found, fallback to full render
	if len(payload.Slots) == 0 && len(payload.HTMLSlots) == 0 && len(textSlots) == 0 && len(htmlSlots) == 0 {
		payload.Full = html
	}

	// Handle list operations if component implements ListProvider
	if lp, ok := component.(core.ListProvider); ok {
		listOps := r.computeListOps(session.SocketID, lp)
		if len(listOps) > 0 {
			payload.ListOps = listOps
		}
	}

	return payload
}

// extractSlotsOptimized extracts data-slot content using O(n) single-pass parsing.
// This is significantly faster than the O(n²) extractSlotsRobust for large HTML.
func extractSlotsOptimized(html string) (textSlots, htmlSlots map[string]string) {
	textSlots = make(map[string]string)
	htmlSlots = make(map[string]string)

	const marker = `data-slot="`
	markerLen := len(marker)
	htmlLen := len(html)
	pos := 0

	for pos < htmlLen {
		// Find next data-slot
		idx := strings.Index(html[pos:], marker)
		if idx == -1 {
			break
		}

		slotStart := pos + idx + markerLen

		// Extract slot ID (until next ")
		slotEnd := strings.IndexByte(html[slotStart:], '"')
		if slotEnd == -1 {
			pos = slotStart
			continue
		}

		slotID := html[slotStart : slotStart+slotEnd]

		// Find the tag start (search backwards for <)
		tagStart := pos + idx
		for tagStart > 0 && html[tagStart] != '<' {
			tagStart--
		}

		// Extract tag name
		tagNameEnd := tagStart + 1
		for tagNameEnd < htmlLen && html[tagNameEnd] != ' ' && html[tagNameEnd] != '>' && html[tagNameEnd] != '/' {
			tagNameEnd++
		}
		tagName := html[tagStart+1 : tagNameEnd]

		// Find the > of the opening tag
		closeAngle := strings.IndexByte(html[slotStart+slotEnd:], '>')
		if closeAngle == -1 {
			pos = slotStart + slotEnd
			continue
		}

		contentStart := slotStart + slotEnd + closeAngle + 1

		// Find matching close tag using depth counter (O(n) for this slot)
		openTag := "<" + tagName
		closeTag := "</" + tagName
		openTagLen := len(openTag)
		closeTagLen := len(closeTag)

		depth := 1
		searchPos := contentStart
		contentEnd := -1

		for depth > 0 && searchPos < htmlLen {
			nextOpen := strings.Index(html[searchPos:], openTag)
			nextClose := strings.Index(html[searchPos:], closeTag)

			if nextClose == -1 {
				break
			}

			// Adjust relative indices
			if nextOpen != -1 {
				nextOpen += searchPos
			} else {
				nextOpen = htmlLen // No more open tags
			}
			nextClose += searchPos

			if nextOpen < nextClose {
				// Check if it's actually a tag (not part of text like "<span")
				afterOpen := nextOpen + openTagLen
				if afterOpen < htmlLen {
					nextChar := html[afterOpen]
					if nextChar == ' ' || nextChar == '>' || nextChar == '/' || nextChar == '\t' || nextChar == '\n' {
						depth++
					}
				}
				searchPos = nextOpen + openTagLen
			} else {
				depth--
				if depth == 0 {
					contentEnd = nextClose
				}
				searchPos = nextClose + closeTagLen
			}
		}

		if contentEnd != -1 {
			content := strings.TrimSpace(html[contentStart:contentEnd])

			// Classify: simple text vs HTML content
			if strings.ContainsAny(content, "<>") {
				htmlSlots[slotID] = content
			} else {
				textSlots[slotID] = content
			}
		}

		pos = searchPos
	}

	return textSlots, htmlSlots
}

// extractSlotsRobust extracts data-slot content supporting nested HTML.
// Returns separate maps for text-only slots and HTML slots.
// Deprecated: Use extractSlotsOptimized for better performance.
func extractSlotsRobust(html string) (textSlots, htmlSlots map[string]string) {
	return extractSlotsOptimized(html)
}

// extractTagName extracts the tag name from tag content.
func extractTagName(s string) string {
	end := strings.IndexAny(s, " \t\n/>")
	if end == -1 {
		return s
	}
	return s[:end]
}

// findContent extracts content until the matching close tag.
func findContent(html, tagName string) string {
	openTag := "<" + tagName
	closeTag := "</" + tagName

	depth := 1
	pos := 0

	for depth > 0 && pos < len(html) {
		nextOpen := strings.Index(html[pos:], openTag)
		nextClose := strings.Index(html[pos:], closeTag)

		if nextClose == -1 {
			return "" // No close tag found
		}

		// Check if there's an open tag before the close tag
		if nextOpen != -1 && nextOpen < nextClose {
			// Check if it's a self-closing tag or actual open
			afterOpen := pos + nextOpen + len(openTag)
			if afterOpen < len(html) {
				nextChar := html[afterOpen]
				if nextChar == ' ' || nextChar == '>' || nextChar == '\t' || nextChar == '\n' {
					depth++
				}
			}
			pos += nextOpen + len(openTag)
		} else {
			depth--
			if depth == 0 {
				return html[:pos+nextClose]
			}
			pos += nextClose + len(closeTag)
		}
	}

	return ""
}

// getAssigns gets the assigns from a component if available.
func (r *Router) getAssigns(component core.Component) *core.Assigns {
	if bc, ok := component.(interface{ Assigns() *core.Assigns }); ok {
		return bc.Assigns()
	}
	return nil
}

// Hash-based slot state cache for O(1) comparison (instead of O(n) string compare)
var (
	slotHashCache   = make(map[string]map[string]uint64)
	slotHashCacheMu sync.RWMutex
)

// hashSlotContent computes FNV-64a hash of content for fast comparison
func hashSlotContent(content string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(content))
	return h.Sum64()
}

// Slot state cache for computing diffs (string content for sending to client)
var (
	slotStateCache   = make(map[string]map[string]string)
	slotStateCacheMu sync.RWMutex
)

// getSlotState retrieves the previous slot state for a socket.
func (r *Router) getSlotState(socketID string) map[string]string {
	slotStateCacheMu.RLock()
	defer slotStateCacheMu.RUnlock()
	return slotStateCache[socketID]
}

// setSlotState stores the current slot state for a socket.
func (r *Router) setSlotState(socketID string, slots map[string]string) {
	slotStateCacheMu.Lock()
	defer slotStateCacheMu.Unlock()
	slotStateCache[socketID] = slots
}

// clearSlotState removes slot state for a socket (called on disconnect).
func (r *Router) clearSlotState(socketID string) {
	slotStateCacheMu.Lock()
	defer slotStateCacheMu.Unlock()
	delete(slotStateCache, socketID)
}

// List state cache for computing list diffs
var (
	listStateCache   = make(map[string]map[string][]core.ListItem)
	listStateCacheMu sync.RWMutex
)

// computeListOps computes list operations for all lists.
func (r *Router) computeListOps(socketID string, lp core.ListProvider) map[string][]core.ListOp {
	result := make(map[string][]core.ListOp)

	lists := lp.GetLists()
	if len(lists) == 0 {
		return nil
	}

	listStateCacheMu.Lock()
	prevLists := listStateCache[socketID]
	if prevLists == nil {
		prevLists = make(map[string][]core.ListItem)
	}

	for listID, items := range lists {
		prevItems := prevLists[listID]
		ops := computeListDiff(prevItems, items)
		if len(ops) > 0 {
			result[listID] = ops
		}
		prevLists[listID] = items
	}

	listStateCache[socketID] = prevLists
	listStateCacheMu.Unlock()

	return result
}

// computeListDiff generates minimal operations to transform prev into next.
func computeListDiff(prev, next []core.ListItem) []core.ListOp {
	var ops []core.ListOp

	// Build maps for quick lookup
	prevMap := make(map[string]int)
	prevContent := make(map[string]string)
	for i, item := range prev {
		prevMap[item.Key] = i
		prevContent[item.Key] = item.Content
	}

	nextMap := make(map[string]int)
	for i, item := range next {
		nextMap[item.Key] = i
	}

	// Detect deletions
	for key := range prevMap {
		if _, ok := nextMap[key]; !ok {
			ops = append(ops, core.ListOp{
				Op:  "d",
				Key: key,
			})
		}
	}

	// Detect insertions and updates
	for i, item := range next {
		if _, existed := prevMap[item.Key]; !existed {
			// Insert
			ops = append(ops, core.ListOp{
				Op:      "i",
				Key:     item.Key,
				Index:   i,
				Content: item.Content,
			})
		} else if prevContent[item.Key] != item.Content {
			// Update
			ops = append(ops, core.ListOp{
				Op:      "u",
				Key:     item.Key,
				Content: item.Content,
			})
		}
	}

	// Detect moves (if order changed but no other changes)
	if len(ops) == 0 && len(prev) == len(next) {
		for i, item := range next {
			if prevIdx, ok := prevMap[item.Key]; ok && prevIdx != i {
				ops = append(ops, core.ListOp{
					Op:    "m",
					Key:   item.Key,
					Index: i,
				})
			}
		}
	}

	return ops
}

// clearListState removes list state for a socket (called on disconnect).
func (r *Router) clearListState(socketID string) {
	listStateCacheMu.Lock()
	defer listStateCacheMu.Unlock()
	delete(listStateCache, socketID)
}

// handleDisconnect handles client disconnection.
func (r *Router) handleDisconnect(session *LiveViewSession) {
	// Terminate component
	ctx := context.Background()
	session.Component.Terminate(ctx, core.TerminateShutdown)

	// Remove from managers
	r.sessionManager.Remove(session.ID)
	r.socketManager.Remove(session.SocketID)

	// Invalidate diff cache
	r.diffEngine.InvalidateSocket(session.SocketID)

	// Clear slot and list state caches
	r.clearSlotState(session.SocketID)
	r.clearListState(session.SocketID)

	// Clear hash cache (new optimization)
	r.clearSlotHashCache(session.SocketID)

	// Close transport
	if session.Transport != nil {
		session.Transport.Close()
	}
}

// clearSlotHashCache removes hash cache for a socket (called on disconnect).
func (r *Router) clearSlotHashCache(socketID string) {
	slotHashCacheMu.Lock()
	defer slotHashCacheMu.Unlock()
	delete(slotHashCache, socketID)
}

// sendReply sends a reply message to the client.
func (r *Router) sendReply(session *LiveViewSession, ref, topic string, response map[string]any) {
	payload := map[string]any{
		"status":   "ok",
		"response": response,
	}

	msg := transport.Message{
		Ref:     ref,
		Topic:   topic,
		Event:   "phx_reply",
		Payload: payload,
	}

	if transport.DebugWebSocket {
		fmt.Printf("[ROUTER DEBUG] sendReply: ref=%s, topic=%s, event=%s\n", ref, topic, msg.Event)
	}

	if err := session.Transport.Send(msg); err != nil {
		if transport.DebugWebSocket {
			fmt.Printf("[ROUTER DEBUG] sendReply error: %v\n", err)
		}
	}
}

// sendError sends an error reply to the client.
func (r *Router) sendError(session *LiveViewSession, ref, topic string, err error) {
	payload := map[string]any{
		"status": "error",
		"response": map[string]any{
			"reason": err.Error(),
		},
	}

	msg := transport.Message{
		Ref:     ref,
		Topic:   topic,
		Event:   "phx_reply",
		Payload: payload,
	}

	session.Transport.Send(msg)
}

// extractSession extracts session data from the request.
func (r *Router) extractSession(req *http.Request) core.Session {
	session := make(core.Session)

	// Extract auth context if available
	if auth := security.AuthFromContext(req.Context()); auth != nil {
		session["user_id"] = auth.UserID
		session["username"] = auth.Username
		session["email"] = auth.Email
		session["roles"] = auth.Roles
		session["session_id"] = auth.SessionID
	}

	// Extract cookies
	for _, cookie := range req.Cookies() {
		session["cookie:"+cookie.Name] = cookie.Value
	}

	return session
}

// extractParams extracts URL parameters and query strings.
func extractParams(req *http.Request) core.Params {
	params := make(core.Params)

	// Add query parameters
	for key, values := range req.URL.Query() {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}

	return params
}

// isWebSocketRequest checks if this is a WebSocket upgrade request.
func isWebSocketRequest(req *http.Request) bool {
	return strings.Contains(strings.ToLower(req.Header.Get("Upgrade")), "websocket")
}

// RouteGroup represents a group of routes with shared prefix/middleware.
type RouteGroup struct {
	router     *Router
	prefix     string
	middleware []Middleware
}

// Use adds middleware to the group.
func (g *RouteGroup) Use(mw Middleware) {
	g.middleware = append(g.middleware, mw)
}

// Live registers a LiveView route in the group.
func (g *RouteGroup) Live(path string, component func() core.Component, opts ...RouteOption) {
	fullPath := g.prefix + path
	route := &LiveRoute{
		Path:       fullPath,
		Component:  component,
		Middleware: make([]Middleware, len(g.middleware)),
		Meta:       make(map[string]any),
	}
	copy(route.Middleware, g.middleware)

	for _, opt := range opts {
		opt(route)
	}

	g.router.mu.Lock()
	g.router.liveRoutes[fullPath] = route
	g.router.mu.Unlock()

	g.router.mux.HandleFunc(fullPath, g.router.handleLive(route))
}

// Handle registers a handler in the group.
func (g *RouteGroup) Handle(pattern string, handler http.Handler) {
	fullPath := g.prefix + pattern

	// Apply group middleware
	h := handler
	for i := len(g.middleware) - 1; i >= 0; i-- {
		h = g.middleware[i](h)
	}

	g.router.Handle(fullPath, h)
}

// Get registers a GET handler.
func (g *RouteGroup) Get(pattern string, handler http.HandlerFunc) {
	fullPath := g.prefix + pattern
	g.router.Handle("GET "+fullPath, handler)
}

// Post registers a POST handler.
func (g *RouteGroup) Post(pattern string, handler http.HandlerFunc) {
	fullPath := g.prefix + pattern
	g.router.Handle("POST "+fullPath, handler)
}

// Put registers a PUT handler.
func (g *RouteGroup) Put(pattern string, handler http.HandlerFunc) {
	fullPath := g.prefix + pattern
	g.router.Handle("PUT "+fullPath, handler)
}

// Delete registers a DELETE handler.
func (g *RouteGroup) Delete(pattern string, handler http.HandlerFunc) {
	fullPath := g.prefix + pattern
	g.router.Handle("DELETE "+fullPath, handler)
}

// methodHandler restricts a handler to a specific HTTP method.
func methodHandler(method string, handler http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handler(w, r)
	})
}

// RouteOption configures a LiveRoute.
type RouteOption func(*LiveRoute)

// WithLayout sets the layout component.
func WithLayout(layout func() core.Component) RouteOption {
	return func(r *LiveRoute) {
		r.Layout = layout
	}
}

// WithHooks sets the plugin hooks.
func WithHooks(hooks ...string) RouteOption {
	return func(r *LiveRoute) {
		r.Hooks = hooks
	}
}

// WithRouteMiddleware adds middleware to the route.
func WithRouteMiddleware(mw ...Middleware) RouteOption {
	return func(r *LiveRoute) {
		r.Middleware = append(r.Middleware, mw...)
	}
}

// WithMeta adds metadata to the route.
func WithMeta(key string, value any) RouteOption {
	return func(r *LiveRoute) {
		r.Meta[key] = value
	}
}

// Context helpers

type routeContextKey struct{}

// WithRouteContext adds route information to context.
func WithRouteContext(ctx context.Context, route *LiveRoute) context.Context {
	return context.WithValue(ctx, routeContextKey{}, route)
}

// RouteFromContext retrieves route information from context.
func RouteFromContext(ctx context.Context) *LiveRoute {
	r, _ := ctx.Value(routeContextKey{}).(*LiveRoute)
	return r
}
