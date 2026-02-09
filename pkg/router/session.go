// Package router provides HTTP routing for GoliveKit applications.
package router

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/gabrielmiguelok/golivekit/pkg/core"
	"github.com/gabrielmiguelok/golivekit/pkg/diff"
	"github.com/gabrielmiguelok/golivekit/pkg/protocol"
	"github.com/gabrielmiguelok/golivekit/pkg/transport"
)

// LiveViewSession vincula HTTP session con WebSocket connection.
type LiveViewSession struct {
	// ID es el identificador único de la sesión
	ID string

	// SocketID es el ID del socket asociado
	SocketID string

	// Component es la instancia del componente LiveView
	Component core.Component

	// Socket es la conexión WebSocket
	Socket *core.Socket

	// Transport es el transporte WebSocket subyacente
	Transport *transport.WebSocketTransport

	// Params son los parámetros de la URL
	Params core.Params

	// Session contiene datos de sesión del usuario
	Session core.Session

	// DiffEngine es el motor de diff para esta sesión
	DiffEngine *diff.Engine

	// Codec es el codec para serialización de mensajes
	Codec protocol.Codec

	// JoinRef es la referencia del join para el protocolo Phoenix
	JoinRef string

	// Topic es el topic del canal Phoenix
	Topic string

	// CreatedAt es cuando se creó la sesión
	CreatedAt time.Time

	// LastActivity es la última actividad de la sesión
	LastActivity time.Time

	// Mounted indica si el componente ya fue montado
	Mounted bool

	// Version es la versión de diff para ordenamiento en el cliente
	Version uint64

	// Per-socket slot state (avoids global lock contention)
	slotHashes map[string]uint64
	slotMu     sync.RWMutex

	mu sync.RWMutex
}

// GetSlotHashes returns the slot hashes for this session (per-socket, no global lock).
func (s *LiveViewSession) GetSlotHashes() map[string]uint64 {
	s.slotMu.RLock()
	defer s.slotMu.RUnlock()
	return s.slotHashes
}

// SetSlotHashes stores the slot hashes for this session (per-socket, no global lock).
func (s *LiveViewSession) SetSlotHashes(hashes map[string]uint64) {
	s.slotMu.Lock()
	defer s.slotMu.Unlock()
	s.slotHashes = hashes
}

// NewLiveViewSession crea una nueva sesión LiveView.
func NewLiveViewSession(socketID string, comp core.Component, params core.Params, session core.Session) *LiveViewSession {
	now := time.Now()
	return &LiveViewSession{
		ID:           generateSessionID(),
		SocketID:     socketID,
		Component:    comp,
		Params:       params,
		Session:      session,
		Topic:        "lv:" + socketID,
		CreatedAt:    now,
		LastActivity: now,
	}
}

// UpdateActivity actualiza el timestamp de última actividad.
func (s *LiveViewSession) UpdateActivity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActivity = time.Now()
}

// GetLastActivity retorna el último timestamp de actividad.
func (s *LiveViewSession) GetLastActivity() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastActivity
}

// SetMounted marca la sesión como montada.
func (s *LiveViewSession) SetMounted(mounted bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Mounted = mounted
}

// IsMounted retorna si la sesión ya fue montada.
func (s *LiveViewSession) IsMounted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Mounted
}

// SetJoinRef establece la referencia del join.
func (s *LiveViewSession) SetJoinRef(ref string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.JoinRef = ref
}

// GetJoinRef retorna la referencia del join.
func (s *LiveViewSession) GetJoinRef() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.JoinRef
}

// LiveViewSessionManager gestiona todas las sesiones LiveView activas.
type LiveViewSessionManager struct {
	// sessions almacena sesiones por ID
	sessions map[string]*LiveViewSession

	// bySocket permite buscar sesiones por socket ID
	bySocket map[string]*LiveViewSession

	// maxSessions es el límite máximo de sesiones (0 = sin límite)
	maxSessions int

	// sessionTTL es el tiempo de vida de sesiones inactivas
	sessionTTL time.Duration

	mu sync.RWMutex
}

// LiveViewSessionManagerConfig configura el session manager.
type LiveViewSessionManagerConfig struct {
	MaxSessions int
	SessionTTL  time.Duration
}

// DefaultSessionManagerConfig retorna la configuración por defecto.
func DefaultSessionManagerConfig() *LiveViewSessionManagerConfig {
	return &LiveViewSessionManagerConfig{
		MaxSessions: 10000,
		SessionTTL:  30 * time.Minute,
	}
}

// NewLiveViewSessionManager crea un nuevo manager de sesiones.
func NewLiveViewSessionManager() *LiveViewSessionManager {
	return NewLiveViewSessionManagerWithConfig(DefaultSessionManagerConfig())
}

// NewLiveViewSessionManagerWithConfig crea un manager con configuración personalizada.
func NewLiveViewSessionManagerWithConfig(config *LiveViewSessionManagerConfig) *LiveViewSessionManager {
	if config == nil {
		config = DefaultSessionManagerConfig()
	}
	return &LiveViewSessionManager{
		sessions:    make(map[string]*LiveViewSession),
		bySocket:    make(map[string]*LiveViewSession),
		maxSessions: config.MaxSessions,
		sessionTTL:  config.SessionTTL,
	}
}

// Create crea y registra una nueva sesión LiveView.
func (m *LiveViewSessionManager) Create(socketID string, comp core.Component, params core.Params, session core.Session) *LiveViewSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Verificar límite de sesiones
	if m.maxSessions > 0 && len(m.sessions) >= m.maxSessions {
		// Eliminar sesiones más antiguas
		m.evictOldestLocked()
	}

	lvSession := NewLiveViewSession(socketID, comp, params, session)
	m.sessions[lvSession.ID] = lvSession
	m.bySocket[socketID] = lvSession

	return lvSession
}

// Get obtiene una sesión por ID.
func (m *LiveViewSessionManager) Get(sessionID string) (*LiveViewSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionID]
	return s, ok
}

// GetBySocket obtiene una sesión por socket ID.
func (m *LiveViewSessionManager) GetBySocket(socketID string) (*LiveViewSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.bySocket[socketID]
	return s, ok
}

// Remove elimina una sesión.
func (m *LiveViewSessionManager) Remove(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[sessionID]; ok {
		delete(m.bySocket, s.SocketID)
		delete(m.sessions, sessionID)
	}
}

// RemoveBySocket elimina una sesión por socket ID.
func (m *LiveViewSessionManager) RemoveBySocket(socketID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.bySocket[socketID]; ok {
		delete(m.sessions, s.ID)
		delete(m.bySocket, socketID)
	}
}

// Count retorna el número de sesiones activas.
func (m *LiveViewSessionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// All retorna todas las sesiones.
func (m *LiveViewSessionManager) All() []*LiveViewSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*LiveViewSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

// Cleanup elimina sesiones inactivas.
func (m *LiveViewSessionManager) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	removed := 0

	for id, s := range m.sessions {
		if now.Sub(s.GetLastActivity()) > m.sessionTTL {
			delete(m.bySocket, s.SocketID)
			delete(m.sessions, id)
			removed++
		}
	}

	return removed
}

// evictOldestLocked elimina las sesiones más antiguas (debe llamarse con lock).
func (m *LiveViewSessionManager) evictOldestLocked() {
	var oldest *LiveViewSession
	var oldestID string

	for id, s := range m.sessions {
		if oldest == nil || s.GetLastActivity().Before(oldest.GetLastActivity()) {
			oldest = s
			oldestID = id
		}
	}

	if oldest != nil {
		delete(m.bySocket, oldest.SocketID)
		delete(m.sessions, oldestID)
	}
}

// StartCleanupRoutine inicia una rutina de limpieza periódica.
func (m *LiveViewSessionManager) StartCleanupRoutine(interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.Cleanup()
			case <-stopCh:
				return
			}
		}
	}()
}

// generateSessionID genera un ID único para la sesión.
func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generateSocketID genera un ID único para el socket.
func generateSocketID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "lv:" + hex.EncodeToString(b)
}
