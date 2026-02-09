// Package a11y provides accessibility utilities for GoliveKit.
package a11y

import (
	"context"
	"fmt"
	"sync"

	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// LiveRegion represents an ARIA live region for announcements.
type LiveRegion struct {
	// ID is the unique identifier for this region.
	ID string

	// Politeness determines how urgently the announcement is made.
	// Options: "polite" (default), "assertive"
	Politeness string

	// Atomic determines if the whole region is announced.
	Atomic bool

	// Relevant specifies what changes trigger announcements.
	// Options: "additions", "removals", "text", "all"
	Relevant string
}

// NewLiveRegion creates a new live region.
func NewLiveRegion(id string, opts ...LiveRegionOption) *LiveRegion {
	lr := &LiveRegion{
		ID:         id,
		Politeness: "polite",
		Atomic:     false,
		Relevant:   "additions text",
	}

	for _, opt := range opts {
		opt(lr)
	}

	return lr
}

// LiveRegionOption configures a live region.
type LiveRegionOption func(*LiveRegion)

// WithPoliteness sets the politeness level.
func WithPoliteness(level string) LiveRegionOption {
	return func(lr *LiveRegion) {
		lr.Politeness = level
	}
}

// Assertive makes the region assertive.
func Assertive() LiveRegionOption {
	return func(lr *LiveRegion) {
		lr.Politeness = "assertive"
	}
}

// Atomic makes the region atomic.
func Atomic() LiveRegionOption {
	return func(lr *LiveRegion) {
		lr.Atomic = true
	}
}

// WithRelevant sets what changes are relevant.
func WithRelevant(relevant string) LiveRegionOption {
	return func(lr *LiveRegion) {
		lr.Relevant = relevant
	}
}

// Announce sends a message to the live region.
func (lr *LiveRegion) Announce(socket *core.Socket, message string) error {
	return socket.Push("live_region_announce", map[string]any{
		"id":      lr.ID,
		"message": message,
	})
}

// AnnounceWithPoliteness sends a message with specific politeness.
func (lr *LiveRegion) AnnounceWithPoliteness(socket *core.Socket, message, politeness string) error {
	return socket.Push("live_region_announce", map[string]any{
		"id":         lr.ID,
		"message":    message,
		"politeness": politeness,
	})
}

// RenderHTML generates the HTML for the live region.
func (lr *LiveRegion) RenderHTML() string {
	atomicAttr := ""
	if lr.Atomic {
		atomicAttr = ` aria-atomic="true"`
	}

	return fmt.Sprintf(
		`<div id="%s" role="status" aria-live="%s" aria-relevant="%s"%s class="sr-only"></div>`,
		lr.ID, lr.Politeness, lr.Relevant, atomicAttr,
	)
}

// FocusManager manages focus state for accessibility.
type FocusManager struct {
	socket      *core.Socket
	focusStack  []string
	trapElement string
	mu          sync.Mutex
}

// NewFocusManager creates a new focus manager.
func NewFocusManager(socket *core.Socket) *FocusManager {
	return &FocusManager{
		socket:     socket,
		focusStack: make([]string, 0),
	}
}

// Focus sets focus on an element.
func (fm *FocusManager) Focus(selector string) error {
	return fm.socket.Push("js_exec", map[string]any{
		"code": fmt.Sprintf(`document.querySelector("%s")?.focus()`, selector),
	})
}

// FocusFirst focuses the first focusable element in a container.
func (fm *FocusManager) FocusFirst(containerSelector string) error {
	code := fmt.Sprintf(`
		const container = document.querySelector("%s");
		if (container) {
			const focusable = container.querySelector('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])');
			if (focusable) focusable.focus();
		}
	`, containerSelector)
	return fm.socket.Push("js_exec", map[string]any{"code": code})
}

// PushFocus saves current focus and sets new focus.
func (fm *FocusManager) PushFocus(selector string) error {
	fm.mu.Lock()
	fm.focusStack = append(fm.focusStack, "document.activeElement")
	fm.mu.Unlock()

	code := fmt.Sprintf(`
		window.__focusStack = window.__focusStack || [];
		window.__focusStack.push(document.activeElement);
		document.querySelector("%s")?.focus();
	`, selector)
	return fm.socket.Push("js_exec", map[string]any{"code": code})
}

// PopFocus restores previous focus.
func (fm *FocusManager) PopFocus() error {
	fm.mu.Lock()
	if len(fm.focusStack) > 0 {
		fm.focusStack = fm.focusStack[:len(fm.focusStack)-1]
	}
	fm.mu.Unlock()

	code := `
		window.__focusStack = window.__focusStack || [];
		const prev = window.__focusStack.pop();
		if (prev) prev.focus();
	`
	return fm.socket.Push("js_exec", map[string]any{"code": code})
}

// TrapFocus traps focus within a container (for modals).
func (fm *FocusManager) TrapFocus(containerSelector string) error {
	fm.mu.Lock()
	fm.trapElement = containerSelector
	fm.mu.Unlock()

	code := fmt.Sprintf(`
		window.__trapFocus = function(container) {
			const focusable = container.querySelectorAll('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])');
			const first = focusable[0];
			const last = focusable[focusable.length - 1];

			container.addEventListener('keydown', function(e) {
				if (e.key !== 'Tab') return;

				if (e.shiftKey) {
					if (document.activeElement === first) {
						e.preventDefault();
						last.focus();
					}
				} else {
					if (document.activeElement === last) {
						e.preventDefault();
						first.focus();
					}
				}
			});

			first?.focus();
		};
		const container = document.querySelector("%s");
		if (container) window.__trapFocus(container);
	`, containerSelector)
	return fm.socket.Push("js_exec", map[string]any{"code": code})
}

// ReleaseFocusTrap releases the focus trap.
func (fm *FocusManager) ReleaseFocusTrap() error {
	fm.mu.Lock()
	fm.trapElement = ""
	fm.mu.Unlock()

	return fm.PopFocus()
}

// RestoreFocus restores focus to a previously focused element.
func (fm *FocusManager) RestoreFocus() error {
	return fm.socket.Push("focus_restore", nil)
}

// MoveFocus moves focus to an element.
func (fm *FocusManager) MoveFocus(selector string) error {
	return fm.Focus(selector)
}

// Announcer provides a simple API for screen reader announcements.
type Announcer struct {
	region *LiveRegion
	socket *core.Socket
}

// NewAnnouncer creates a new announcer.
func NewAnnouncer(socket *core.Socket) *Announcer {
	return &Announcer{
		region: NewLiveRegion("announcer"),
		socket: socket,
	}
}

// Announce makes a polite announcement.
func (a *Announcer) Announce(message string) error {
	return a.region.Announce(a.socket, message)
}

// AnnounceUrgent makes an assertive announcement.
func (a *Announcer) AnnounceUrgent(message string) error {
	return a.region.AnnounceWithPoliteness(a.socket, message, "assertive")
}

// RenderHTML returns the HTML for the announcer.
func (a *Announcer) RenderHTML() string {
	return a.region.RenderHTML()
}

// Context helpers

type a11yContextKey struct{}

// WithA11y adds accessibility helpers to context.
func WithA11y(ctx context.Context, socket *core.Socket) context.Context {
	a11y := &A11yHelpers{
		Announcer:    NewAnnouncer(socket),
		FocusManager: NewFocusManager(socket),
	}
	return context.WithValue(ctx, a11yContextKey{}, a11y)
}

// A11yFromContext retrieves accessibility helpers from context.
func A11yFromContext(ctx context.Context) *A11yHelpers {
	a, _ := ctx.Value(a11yContextKey{}).(*A11yHelpers)
	return a
}

// A11yHelpers bundles accessibility utilities.
type A11yHelpers struct {
	Announcer    *Announcer
	FocusManager *FocusManager
}

// SkipLink generates a skip link for keyboard navigation.
func SkipLink(target, text string) string {
	return fmt.Sprintf(
		`<a href="#%s" class="sr-only focus:not-sr-only focus:absolute focus:top-0 focus:left-0 focus:z-50 focus:p-4 focus:bg-white focus:text-black">%s</a>`,
		target, text,
	)
}

// SROnly returns CSS class for screen-reader-only content.
func SROnly() string {
	return "sr-only"
}

// AriaLabel generates an aria-label attribute.
func AriaLabel(label string) string {
	return fmt.Sprintf(`aria-label="%s"`, label)
}

// AriaDescribedBy generates an aria-describedby attribute.
func AriaDescribedBy(id string) string {
	return fmt.Sprintf(`aria-describedby="%s"`, id)
}

// AriaExpanded generates an aria-expanded attribute.
func AriaExpanded(expanded bool) string {
	return fmt.Sprintf(`aria-expanded="%t"`, expanded)
}

// AriaHidden generates an aria-hidden attribute.
func AriaHidden(hidden bool) string {
	return fmt.Sprintf(`aria-hidden="%t"`, hidden)
}

// Role generates a role attribute.
func Role(role string) string {
	return fmt.Sprintf(`role="%s"`, role)
}
