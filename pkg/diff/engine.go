package diff

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// Engine combines compile-time and runtime analysis to generate minimal diffs.
type Engine struct {
	compiler *Compiler

	// Cache of render states per socket
	renderCache sync.Map // socketID -> *RenderState

	// Metrics
	metrics *Metrics
}

// RenderState tracks the render state for a socket.
type RenderState struct {
	// AST of the current template
	ast *TemplateAST

	// Current values of each slot
	slots map[string]SlotState

	// Render version
	version uint64

	// Last render time
	lastRender time.Time

	mu sync.RWMutex
}

// SlotState holds the current state of a slot.
type SlotState struct {
	Content  []byte
	Hash     uint64
	Children map[string]SlotState // For nested loops
}

// Metrics tracks diff engine performance using atomic operations for thread-safety.
type Metrics struct {
	DiffCount     atomic.Int64
	DiffLatency   atomic.Int64 // stored as nanoseconds
	DiffTotalSize atomic.Int64
	SlotsUpdated  atomic.Int64
	CacheHits     atomic.Int64
	CacheMisses   atomic.Int64
}

// NewEngine creates a new diff engine.
func NewEngine() *Engine {
	return &Engine{
		compiler: NewCompiler(),
		metrics:  &Metrics{},
	}
}

// Compiler returns the template compiler.
func (e *Engine) Compiler() *Compiler {
	return e.compiler
}

// ComputeDiff generates the minimal diff between renders.
func (e *Engine) ComputeDiff(socketID string, ast *TemplateAST, assigns *core.Assigns, render func(slotID string) ([]byte, error)) (*Diff, error) {
	start := time.Now()
	defer func() {
		e.metrics.DiffCount.Add(1)
		e.metrics.DiffLatency.Add(int64(time.Since(start)))
	}()

	// Get or create render state
	stateI, loaded := e.renderCache.LoadOrStore(socketID, &RenderState{
		ast:        ast,
		slots:      make(map[string]SlotState),
		lastRender: time.Now(),
	})
	state := stateI.(*RenderState)

	if loaded {
		e.metrics.CacheHits.Add(1)
	} else {
		e.metrics.CacheMisses.Add(1)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	// Check if template changed
	if state.ast != nil && state.ast.Fingerprint != ast.Fingerprint {
		// Template structure changed, need full re-render
		state.ast = ast
		state.slots = make(map[string]SlotState)
	}

	// Get changed fields from assigns
	changedFields := assigns.Tracker().GetChanged()

	if len(changedFields) == 0 && len(state.slots) > 0 {
		return nil, nil // No changes
	}

	// Determine which slots need re-render
	affectedSlots := e.findAffectedSlots(ast, changedFields)

	// If no affected slots but state is empty, render all
	if len(affectedSlots) == 0 && len(state.slots) == 0 {
		for _, slot := range ast.Slots {
			affectedSlots = append(affectedSlots, slot.ID)
		}
	}

	if len(affectedSlots) == 0 {
		return nil, nil // Changes don't affect any slot
	}

	// Generate new content only for affected slots
	diff := &Diff{
		SocketID: socketID,
		Version:  state.version + 1,
		Slots:    make(map[string]string),
	}

	for _, slotID := range affectedSlots {
		// Render slot
		content, err := render(slotID)
		if err != nil {
			return nil, fmt.Errorf("render slot %s: %w", slotID, err)
		}

		contentHash := hashBytes(content)

		// Check if actually changed
		if prev, ok := state.slots[slotID]; ok && prev.Hash == contentHash {
			continue // No change
		}

		diff.Slots[slotID] = string(content)
		state.slots[slotID] = SlotState{
			Content: content,
			Hash:    contentHash,
		}
	}

	if len(diff.Slots) == 0 {
		return nil, nil
	}

	state.version++
	state.lastRender = time.Now()

	e.metrics.DiffTotalSize.Add(int64(diff.Size()))
	e.metrics.SlotsUpdated.Add(int64(len(diff.Slots)))

	return diff, nil
}

// findAffectedSlots determines which slots depend on the changed fields.
func (e *Engine) findAffectedSlots(ast *TemplateAST, changedFields []string) []string {
	if len(changedFields) == 0 {
		return nil
	}

	changedSet := make(map[string]bool)
	for _, f := range changedFields {
		changedSet[f] = true
		// Also mark parent fields (User.Name -> User)
		parts := strings.Split(f, ".")
		for i := 1; i < len(parts); i++ {
			changedSet[strings.Join(parts[:i], ".")] = true
		}
	}

	var affected []string
	seen := make(map[string]bool)

	for _, slot := range ast.Slots {
		for _, dep := range slot.DependsOn {
			if changedSet[dep] && !seen[slot.ID] {
				affected = append(affected, slot.ID)
				seen[slot.ID] = true
				break
			}
		}
	}

	return affected
}

// FullRender generates a complete render (no diffing).
func (e *Engine) FullRender(socketID string, ast *TemplateAST, renderAll func() (string, error)) (*Diff, error) {
	start := time.Now()
	defer func() {
		e.metrics.DiffCount.Add(1)
		e.metrics.DiffLatency.Add(int64(time.Since(start)))
	}()

	content, err := renderAll()
	if err != nil {
		return nil, err
	}

	// Update state
	stateI, _ := e.renderCache.LoadOrStore(socketID, &RenderState{
		ast:        ast,
		slots:      make(map[string]SlotState),
		lastRender: time.Now(),
	})
	state := stateI.(*RenderState)

	state.mu.Lock()
	state.version++
	state.lastRender = time.Now()
	state.mu.Unlock()

	return &Diff{
		SocketID:        socketID,
		Version:         state.version,
		FullRender:      content,
		TemplateChanged: true,
	}, nil
}

// Diff represents a set of changes to send to the client.
type Diff struct {
	SocketID        string            `json:"-"`
	Version         uint64            `json:"v"`
	Slots           map[string]string `json:"s,omitempty"`
	FullRender      string            `json:"f,omitempty"`
	TemplateChanged bool              `json:"tc,omitempty"`
}

// Size returns the total size of the diff in bytes.
func (d *Diff) Size() int {
	size := 0
	for _, content := range d.Slots {
		size += len(content)
	}
	size += len(d.FullRender)
	return size
}

// IsEmpty returns true if there are no changes.
func (d *Diff) IsEmpty() bool {
	return len(d.Slots) == 0 && len(d.FullRender) == 0
}

// InvalidateSocket removes cached state for a socket.
func (e *Engine) InvalidateSocket(socketID string) {
	e.renderCache.Delete(socketID)
}

// GetState returns the render state for a socket.
func (e *Engine) GetState(socketID string) (*RenderState, bool) {
	if state, ok := e.renderCache.Load(socketID); ok {
		return state.(*RenderState), true
	}
	return nil, false
}

// Metrics returns the engine metrics.
func (e *Engine) Metrics() *Metrics {
	return e.metrics
}

// MetricsSnapshot returns a snapshot of current metrics as plain values.
type MetricsSnapshot struct {
	DiffCount     int64
	DiffLatency   time.Duration
	DiffTotalSize int64
	SlotsUpdated  int64
	CacheHits     int64
	CacheMisses   int64
}

// Snapshot returns a copy of current metrics.
func (e *Engine) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		DiffCount:     e.metrics.DiffCount.Load(),
		DiffLatency:   time.Duration(e.metrics.DiffLatency.Load()),
		DiffTotalSize: e.metrics.DiffTotalSize.Load(),
		SlotsUpdated:  e.metrics.SlotsUpdated.Load(),
		CacheHits:     e.metrics.CacheHits.Load(),
		CacheMisses:   e.metrics.CacheMisses.Load(),
	}
}

// MetricsSnapshot returns a copy of current metrics (deprecated: use Snapshot()).
// Kept for backwards compatibility.
func (e *Engine) MetricsSnapshot() MetricsSnapshot {
	return e.Snapshot()
}

// hashBytes calculates a fast hash of bytes.
func hashBytes(b []byte) uint64 {
	h := fnv.New64a()
	h.Write(b)
	return h.Sum64()
}

// MergeDiffs combines multiple diffs into one.
func MergeDiffs(diffs ...*Diff) *Diff {
	if len(diffs) == 0 {
		return nil
	}

	result := &Diff{
		Slots: make(map[string]string),
	}

	var maxVersion uint64
	var buf bytes.Buffer

	for _, d := range diffs {
		if d == nil {
			continue
		}

		if d.Version > maxVersion {
			maxVersion = d.Version
			result.SocketID = d.SocketID
		}

		if d.FullRender != "" {
			buf.WriteString(d.FullRender)
			result.TemplateChanged = true
		}

		for slotID, content := range d.Slots {
			result.Slots[slotID] = content
		}
	}

	result.Version = maxVersion
	if buf.Len() > 0 {
		result.FullRender = buf.String()
	}

	return result
}
