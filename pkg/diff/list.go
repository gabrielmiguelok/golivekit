package diff

// ListDiff generates efficient diffs for lists/loops.
// Uses a simplified version of the Myers diff algorithm.
type ListDiff struct {
	// Operations to transform the old list into the new list
	Operations []ListOperation
}

// ListOperation represents a single list transformation.
type ListOperation struct {
	Type    ListOpType
	Key     string // Key of the item
	Index   int    // Position
	Content string // HTML content (only for Insert/Update)
}

// ListOpType indicates the type of list operation.
type ListOpType int

const (
	// ListOpInsert adds a new item
	ListOpInsert ListOpType = iota
	// ListOpDelete removes an item
	ListOpDelete
	// ListOpMove moves an item to a new position
	ListOpMove
	// ListOpUpdate updates an item's content
	ListOpUpdate
)

func (t ListOpType) String() string {
	switch t {
	case ListOpInsert:
		return "insert"
	case ListOpDelete:
		return "delete"
	case ListOpMove:
		return "move"
	case ListOpUpdate:
		return "update"
	default:
		return "unknown"
	}
}

// ListItem represents an item in a list for diffing.
type ListItem struct {
	Key     string // Unique identifier
	Content string // Rendered HTML
	Hash    uint64 // Hash of content for quick comparison
}

// ComputeListDiff generates the minimal operations to transform prev into next.
func ComputeListDiff(prev, next []ListItem) *ListDiff {
	diff := &ListDiff{
		Operations: make([]ListOperation, 0),
	}

	// Create maps for quick lookup
	prevMap := make(map[string]int) // key -> index
	nextMap := make(map[string]int) // key -> index

	for i, item := range prev {
		prevMap[item.Key] = i
	}

	for i, item := range next {
		nextMap[item.Key] = i
	}

	// Detect deletions (in prev but not in next)
	for key := range prevMap {
		if _, ok := nextMap[key]; !ok {
			diff.Operations = append(diff.Operations, ListOperation{
				Type: ListOpDelete,
				Key:  key,
			})
		}
	}

	// Detect insertions and updates
	for i, item := range next {
		prevIdx, existed := prevMap[item.Key]

		if !existed {
			// Insert new item
			diff.Operations = append(diff.Operations, ListOperation{
				Type:    ListOpInsert,
				Key:     item.Key,
				Index:   i,
				Content: item.Content,
			})
		} else if prev[prevIdx].Hash != item.Hash {
			// Update (content changed)
			diff.Operations = append(diff.Operations, ListOperation{
				Type:    ListOpUpdate,
				Key:     item.Key,
				Content: item.Content,
			})
		}
	}

	// Detect moves (simplified - only track if order changed significantly)
	// The client can handle reordering based on the final positions
	if len(diff.Operations) == 0 && !sameOrder(prev, next) {
		// Order changed but no other changes
		// Send all positions for the client to reorder
		for i, item := range next {
			if prevIdx, ok := prevMap[item.Key]; ok && prevIdx != i {
				diff.Operations = append(diff.Operations, ListOperation{
					Type:  ListOpMove,
					Key:   item.Key,
					Index: i,
				})
			}
		}
	}

	return diff
}

// sameOrder checks if two lists have the same order (ignoring content).
func sameOrder(prev, next []ListItem) bool {
	if len(prev) != len(next) {
		return false
	}

	for i := range prev {
		if prev[i].Key != next[i].Key {
			return false
		}
	}

	return true
}

// IsEmpty returns true if there are no operations.
func (d *ListDiff) IsEmpty() bool {
	return len(d.Operations) == 0
}

// Size returns the number of operations.
func (d *ListDiff) Size() int {
	return len(d.Operations)
}

// HasDeletes returns true if there are delete operations.
func (d *ListDiff) HasDeletes() bool {
	for _, op := range d.Operations {
		if op.Type == ListOpDelete {
			return true
		}
	}
	return false
}

// HasInserts returns true if there are insert operations.
func (d *ListDiff) HasInserts() bool {
	for _, op := range d.Operations {
		if op.Type == ListOpInsert {
			return true
		}
	}
	return false
}

// ContentSize returns the total size of content in operations.
func (d *ListDiff) ContentSize() int {
	size := 0
	for _, op := range d.Operations {
		size += len(op.Content)
	}
	return size
}

// Filter returns operations of specific types.
func (d *ListDiff) Filter(types ...ListOpType) []ListOperation {
	typeSet := make(map[ListOpType]bool)
	for _, t := range types {
		typeSet[t] = true
	}

	var result []ListOperation
	for _, op := range d.Operations {
		if typeSet[op.Type] {
			result = append(result, op)
		}
	}
	return result
}

// ListDiffBuilder helps build list items for diffing.
type ListDiffBuilder struct {
	items []ListItem
}

// NewListDiffBuilder creates a new builder.
func NewListDiffBuilder() *ListDiffBuilder {
	return &ListDiffBuilder{
		items: make([]ListItem, 0),
	}
}

// Add appends an item to the list.
func (b *ListDiffBuilder) Add(key, content string) *ListDiffBuilder {
	b.items = append(b.items, ListItem{
		Key:     key,
		Content: content,
		Hash:    hashBytes([]byte(content)),
	})
	return b
}

// Build returns the list items.
func (b *ListDiffBuilder) Build() []ListItem {
	return b.items
}

// Clear resets the builder.
func (b *ListDiffBuilder) Clear() *ListDiffBuilder {
	b.items = b.items[:0]
	return b
}

// KeyedCollection helps manage keyed collections.
type KeyedCollection[T any] struct {
	items    []T
	keyFn    func(T) string
	renderFn func(T) string
}

// NewKeyedCollection creates a new keyed collection.
func NewKeyedCollection[T any](keyFn func(T) string, renderFn func(T) string) *KeyedCollection[T] {
	return &KeyedCollection[T]{
		items:    make([]T, 0),
		keyFn:    keyFn,
		renderFn: renderFn,
	}
}

// Set replaces the collection items.
func (c *KeyedCollection[T]) Set(items []T) {
	c.items = items
}

// ToListItems converts the collection to list items for diffing.
func (c *KeyedCollection[T]) ToListItems() []ListItem {
	result := make([]ListItem, len(c.items))
	for i, item := range c.items {
		content := c.renderFn(item)
		result[i] = ListItem{
			Key:     c.keyFn(item),
			Content: content,
			Hash:    hashBytes([]byte(content)),
		}
	}
	return result
}

// Items returns the raw items.
func (c *KeyedCollection[T]) Items() []T {
	return c.items
}
