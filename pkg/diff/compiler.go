// Package diff provides the hybrid diff engine for GoliveKit.
// It combines compile-time AST analysis with runtime change tracking
// to generate minimal DOM updates.
package diff

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// TemplateAST represents the analyzed structure of a template.
type TemplateAST struct {
	// Name is the unique identifier for this template
	Name string

	// Fingerprint is a SHA256 hash of the static structure
	Fingerprint [32]byte

	// Nodes contains all template nodes (static and dynamic)
	Nodes []ASTNode

	// Slots are the dynamic portions that can change
	Slots []SlotDefinition

	// Dependencies are the assign fields that affect this template
	Dependencies []string
}

// GetSlot retrieves a slot by ID.
func (t *TemplateAST) GetSlot(id string) *SlotDefinition {
	for i := range t.Slots {
		if t.Slots[i].ID == id {
			return &t.Slots[i]
		}
	}
	return nil
}

// SlotDefinition describes a dynamic portion of a template.
type SlotDefinition struct {
	ID        string
	Type      DynamicType
	DependsOn []string // Assign fields that affect this slot
}

// DynamicType indicates the type of dynamic content.
type DynamicType int

const (
	DynamicText DynamicType = iota
	DynamicHTML
	DynamicAttr
	DynamicComponent
	DynamicLoop
	DynamicConditional
)

func (dt DynamicType) String() string {
	switch dt {
	case DynamicText:
		return "text"
	case DynamicHTML:
		return "html"
	case DynamicAttr:
		return "attr"
	case DynamicComponent:
		return "component"
	case DynamicLoop:
		return "loop"
	case DynamicConditional:
		return "conditional"
	default:
		return "unknown"
	}
}

// ASTNode is the interface for all AST nodes.
type ASTNode interface {
	nodeType() string
}

// StaticNode represents static HTML content.
type StaticNode struct {
	Content []byte
}

func (n *StaticNode) nodeType() string { return "static" }

// DynamicNode represents a dynamic expression.
type DynamicNode struct {
	SlotID     string
	Expression string
	Type       DynamicType
	DependsOn  []string
}

func (n *DynamicNode) nodeType() string { return "dynamic" }

// LoopNode represents a for loop in the template.
type LoopNode struct {
	SlotID     string
	Collection string // Field name in assigns
	ItemVar    string // Loop variable name
	KeyExpr    string // Expression for the key (for efficient diffing)
	Body       []ASTNode
}

func (n *LoopNode) nodeType() string { return "loop" }

// ConditionalNode represents an if/else in the template.
type ConditionalNode struct {
	SlotID    string
	Condition string
	Then      []ASTNode
	Else      []ASTNode
}

func (n *ConditionalNode) nodeType() string { return "conditional" }

// Compiler analyzes templates and generates optimized ASTs.
type Compiler struct {
	cache    sync.Map // name -> *TemplateAST
	exprRe   *regexp.Regexp
	fieldRe  *regexp.Regexp
}

// NewCompiler creates a new template compiler.
func NewCompiler() *Compiler {
	return &Compiler{
		// Matches expressions like { .Field } or { props.Name }
		exprRe: regexp.MustCompile(`\{\s*\.?(\w+(?:\.\w+)*)\s*\}`),
		// Matches field references like "props.User.Name"
		fieldRe: regexp.MustCompile(`\.(\w+(?:\.\w+)*)`),
	}
}

// Compile analyzes a template source and generates an AST.
// This is a simplified version - a full implementation would use
// the actual templ parser.
func (c *Compiler) Compile(name string, source string) (*TemplateAST, error) {
	// Check cache
	if cached, ok := c.cache.Load(name); ok {
		return cached.(*TemplateAST), nil
	}

	ast := &TemplateAST{
		Name:  name,
		Nodes: make([]ASTNode, 0),
		Slots: make([]SlotDefinition, 0),
	}

	// Parse the template
	if err := c.parseSource(source, ast); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Compute fingerprint from static parts
	ast.Fingerprint = c.computeFingerprint(ast)

	// Cache the result
	c.cache.Store(name, ast)

	return ast, nil
}

// parseSource parses template source into AST nodes.
func (c *Compiler) parseSource(source string, ast *TemplateAST) error {
	slotIndex := 0
	pos := 0

	for pos < len(source) {
		// Find next expression
		match := c.exprRe.FindStringIndex(source[pos:])

		if match == nil {
			// No more expressions, rest is static
			if pos < len(source) {
				ast.Nodes = append(ast.Nodes, &StaticNode{
					Content: []byte(source[pos:]),
				})
			}
			break
		}

		// Add static content before the expression
		if match[0] > 0 {
			ast.Nodes = append(ast.Nodes, &StaticNode{
				Content: []byte(source[pos : pos+match[0]]),
			})
		}

		// Extract the expression
		exprText := source[pos+match[0] : pos+match[1]]
		deps := c.extractDependencies(exprText)

		slotID := fmt.Sprintf("s%d", slotIndex)
		slotIndex++

		// Add dynamic node
		ast.Nodes = append(ast.Nodes, &DynamicNode{
			SlotID:     slotID,
			Expression: exprText,
			Type:       DynamicText,
			DependsOn:  deps,
		})

		// Add slot definition
		ast.Slots = append(ast.Slots, SlotDefinition{
			ID:        slotID,
			Type:      DynamicText,
			DependsOn: deps,
		})

		// Track dependencies
		ast.Dependencies = appendUnique(ast.Dependencies, deps...)

		pos += match[1]
	}

	return nil
}

// extractDependencies extracts assign field references from an expression.
func (c *Compiler) extractDependencies(expr string) []string {
	var deps []string

	matches := c.fieldRe.FindAllStringSubmatch(expr, -1)
	for _, m := range matches {
		if len(m) > 1 {
			deps = appendUnique(deps, m[1])
		}
	}

	return deps
}

// computeFingerprint calculates a hash of the static structure.
func (c *Compiler) computeFingerprint(ast *TemplateAST) [32]byte {
	h := sha256.New()

	for _, node := range ast.Nodes {
		if static, ok := node.(*StaticNode); ok {
			h.Write(static.Content)
		} else {
			// For dynamic nodes, write a placeholder
			h.Write([]byte{0xFF})
		}
	}

	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result
}

// Get retrieves a cached AST by name.
func (c *Compiler) Get(name string) (*TemplateAST, bool) {
	if cached, ok := c.cache.Load(name); ok {
		return cached.(*TemplateAST), true
	}
	return nil, false
}

// Invalidate removes a cached AST.
func (c *Compiler) Invalidate(name string) {
	c.cache.Delete(name)
}

// Clear removes all cached ASTs.
func (c *Compiler) Clear() {
	c.cache.Range(func(key, _ any) bool {
		c.cache.Delete(key)
		return true
	})
}

// appendUnique appends strings to a slice, avoiding duplicates.
func appendUnique(slice []string, items ...string) []string {
	seen := make(map[string]bool, len(slice))
	for _, s := range slice {
		seen[s] = true
	}

	for _, item := range items {
		if !seen[item] {
			slice = append(slice, item)
			seen[item] = true
		}
	}

	return slice
}

// ASTVisitor is the interface for visiting AST nodes.
type ASTVisitor interface {
	VisitStatic(node *StaticNode) error
	VisitDynamic(node *DynamicNode) error
	VisitLoop(node *LoopNode) error
	VisitConditional(node *ConditionalNode) error
}

// Walk traverses an AST and calls the visitor for each node.
func Walk(nodes []ASTNode, visitor ASTVisitor) error {
	for _, node := range nodes {
		var err error
		switch n := node.(type) {
		case *StaticNode:
			err = visitor.VisitStatic(n)
		case *DynamicNode:
			err = visitor.VisitDynamic(n)
		case *LoopNode:
			err = visitor.VisitLoop(n)
			if err == nil {
				err = Walk(n.Body, visitor)
			}
		case *ConditionalNode:
			err = visitor.VisitConditional(n)
			if err == nil {
				err = Walk(n.Then, visitor)
			}
			if err == nil && len(n.Else) > 0 {
				err = Walk(n.Else, visitor)
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// ExtractDependencies returns all dependencies from an AST.
func ExtractDependencies(ast *TemplateAST) []string {
	deps := make([]string, len(ast.Dependencies))
	copy(deps, ast.Dependencies)
	return deps
}

// SlotsByDependency groups slots by their dependencies.
func SlotsByDependency(ast *TemplateAST) map[string][]string {
	result := make(map[string][]string)

	for _, slot := range ast.Slots {
		for _, dep := range slot.DependsOn {
			// Handle nested dependencies (User.Name -> User)
			parts := strings.Split(dep, ".")
			for i := 1; i <= len(parts); i++ {
				key := strings.Join(parts[:i], ".")
				result[key] = appendUnique(result[key], slot.ID)
			}
		}
	}

	return result
}
