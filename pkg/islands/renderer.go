package islands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"

	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// Renderer handles island rendering.
type Renderer struct {
	registry   *core.ComponentRegistry
	serializer *PropsSerializer
}

// NewRenderer creates a new island renderer.
func NewRenderer(registry *core.ComponentRegistry) *Renderer {
	return &Renderer{
		registry:   registry,
		serializer: NewPropsSerializer(),
	}
}

// RenderIsland renders a component as an island with wrapper.
func (r *Renderer) RenderIsland(ctx context.Context, island *Island, assigns map[string]any) (string, error) {
	// Serialize props for hydration
	propsJSON, err := r.serializer.Serialize(assigns)
	if err != nil {
		return "", fmt.Errorf("serialize props: %w", err)
	}

	// Get component factory
	factory, ok := r.registry.Get(island.Name)
	if !ok {
		return "", fmt.Errorf("component not found: %s", island.Name)
	}

	// Create component instance
	comp := factory()

	// Mount with empty params/session (islands don't have full context)
	if err := comp.Mount(ctx, core.Params{}, core.Session{}); err != nil {
		return "", fmt.Errorf("mount component: %w", err)
	}

	// Set assigns if component supports it
	if assignable, ok := comp.(interface{ Assigns() *core.Assigns }); ok {
		assignable.Assigns().SetAll(assigns)
	}

	// Render HTML
	renderer := comp.Render(ctx)
	var buf bytes.Buffer
	if err := renderer.Render(ctx, &buf); err != nil {
		return "", fmt.Errorf("render component: %w", err)
	}

	// Wrap in island container
	return r.wrapIsland(island, propsJSON, buf.String()), nil
}

// RenderStaticIsland renders an island without hydration support.
func (r *Renderer) RenderStaticIsland(ctx context.Context, island *Island, assigns map[string]any) (string, error) {
	// Get component factory
	factory, ok := r.registry.Get(island.Name)
	if !ok {
		return "", fmt.Errorf("component not found: %s", island.Name)
	}

	// Create and render
	comp := factory()
	if err := comp.Mount(ctx, core.Params{}, core.Session{}); err != nil {
		return "", fmt.Errorf("mount component: %w", err)
	}

	if assignable, ok := comp.(interface{ Assigns() *core.Assigns }); ok {
		assignable.Assigns().SetAll(assigns)
	}

	renderer := comp.Render(ctx)
	var buf bytes.Buffer
	if err := renderer.Render(ctx, &buf); err != nil {
		return "", fmt.Errorf("render component: %w", err)
	}

	// No hydration wrapper for static islands
	return buf.String(), nil
}

// wrapIsland wraps rendered content in an island element.
func (r *Renderer) wrapIsland(island *Island, propsJSON, content string) string {
	attrs := fmt.Sprintf(`id="%s" component="%s" hydrate="%s" priority="%d"`,
		island.ID,
		island.Name,
		island.Hydration,
		island.Priority,
	)

	if island.Hydration != HydrateNever {
		attrs += fmt.Sprintf(` props='%s'`, html.EscapeString(propsJSON))
	}

	return fmt.Sprintf("<golive-island %s>%s</golive-island>", attrs, content)
}

// RenderToWriter renders an island directly to a writer.
func (r *Renderer) RenderToWriter(ctx context.Context, w io.Writer, island *Island, assigns map[string]any) error {
	html, err := r.RenderIsland(ctx, island, assigns)
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(html))
	return err
}

// PropsSerializer handles serialization of island props.
type PropsSerializer struct {
	// MaxDepth limits nesting depth to prevent cycles
	MaxDepth int
}

// NewPropsSerializer creates a new props serializer.
func NewPropsSerializer() *PropsSerializer {
	return &PropsSerializer{
		MaxDepth: 10,
	}
}

// Serialize converts props to JSON string.
func (s *PropsSerializer) Serialize(props map[string]any) (string, error) {
	// Filter out non-serializable values
	filtered := s.filterProps(props, 0)

	data, err := json.Marshal(filtered)
	if err != nil {
		return "{}", err
	}

	return string(data), nil
}

// Deserialize parses JSON string to props.
func (s *PropsSerializer) Deserialize(jsonStr string) (map[string]any, error) {
	var props map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &props); err != nil {
		return nil, err
	}
	return props, nil
}

// filterProps removes non-serializable values.
func (s *PropsSerializer) filterProps(props map[string]any, depth int) map[string]any {
	if depth > s.MaxDepth {
		return nil
	}

	result := make(map[string]any)

	for key, value := range props {
		switch v := value.(type) {
		case nil, bool, int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64,
			float32, float64, string:
			result[key] = v
		case []any:
			filtered := make([]any, 0, len(v))
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					filtered = append(filtered, s.filterProps(m, depth+1))
				} else {
					filtered = append(filtered, item)
				}
			}
			result[key] = filtered
		case map[string]any:
			result[key] = s.filterProps(v, depth+1)
		// Skip functions, channels, and other non-serializable types
		}
	}

	return result
}

// IslandRenderer is a convenience function for inline island rendering.
func RenderInline(ctx context.Context, id, name string, props map[string]any, content string, opts ...IslandOption) string {
	island := NewIsland(id, name, opts...)
	island.Props = props

	propsJSON, _ := json.Marshal(props)

	return fmt.Sprintf(`<golive-island id="%s" component="%s" hydrate="%s" priority="%d" props='%s'>%s</golive-island>`,
		island.ID,
		island.Name,
		island.Hydration,
		island.Priority,
		html.EscapeString(string(propsJSON)),
		content,
	)
}
