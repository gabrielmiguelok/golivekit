package forms

import (
	"fmt"
	"regexp"
	"strings"
)

// Changeset provides an Ecto-inspired way to manage form data and validation.
// It tracks changes, validates data, and can be used for insert/update operations.
type Changeset struct {
	// Data is the original data (e.g., from database).
	Data map[string]any

	// Changes are the modifications to be applied.
	Changes map[string]any

	// Errors contains validation errors keyed by field name.
	Errors map[string][]string

	// Valid indicates if the changeset has passed validation.
	Valid bool

	// Action is the operation type (insert, update, delete).
	Action string

	// Required fields that must be present.
	required []string
}

// NewChangeset creates a new changeset from existing data.
func NewChangeset(data map[string]any) *Changeset {
	dataCopy := make(map[string]any)
	for k, v := range data {
		dataCopy[k] = v
	}

	return &Changeset{
		Data:    dataCopy,
		Changes: make(map[string]any),
		Errors:  make(map[string][]string),
		Valid:   true,
	}
}

// Cast filters and casts input params to changes.
// Only fields in the allowed list are included.
func Cast(data, params map[string]any, allowed []string) *Changeset {
	cs := NewChangeset(data)

	allowedSet := make(map[string]bool)
	for _, field := range allowed {
		allowedSet[field] = true
	}

	for key, value := range params {
		if allowedSet[key] {
			// Check if value is different from existing data
			if data[key] != value {
				cs.Changes[key] = value
			}
		}
	}

	return cs
}

// Change adds a change manually.
func (cs *Changeset) Change(key string, value any) *Changeset {
	cs.Changes[key] = value
	return cs
}

// GetChange retrieves a change value.
func (cs *Changeset) GetChange(key string) (any, bool) {
	v, ok := cs.Changes[key]
	return v, ok
}

// GetField retrieves a field value (change if present, otherwise data).
func (cs *Changeset) GetField(key string) any {
	if v, ok := cs.Changes[key]; ok {
		return v
	}
	return cs.Data[key]
}

// GetString retrieves a string field.
func (cs *Changeset) GetString(key string) string {
	if v, ok := cs.GetField(key).(string); ok {
		return v
	}
	return ""
}

// GetInt retrieves an int field.
func (cs *Changeset) GetInt(key string) int {
	switch v := cs.GetField(key).(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// ValidateRequired validates that required fields are present.
func (cs *Changeset) ValidateRequired(fields ...string) *Changeset {
	for _, field := range fields {
		value := cs.GetField(field)
		if isEmpty(value) {
			cs.AddError(field, "is required")
		}
	}
	return cs
}

// ValidateFormat validates a field matches a regex pattern.
func (cs *Changeset) ValidateFormat(field, pattern string, opts ...FormatOption) *Changeset {
	value := cs.GetString(field)
	if value == "" {
		return cs // Skip empty values
	}

	matched, err := regexp.MatchString(pattern, value)
	if err != nil || !matched {
		msg := "has invalid format"
		for _, opt := range opts {
			if opt.message != "" {
				msg = opt.message
			}
		}
		cs.AddError(field, msg)
	}

	return cs
}

// FormatOption configures format validation.
type FormatOption struct {
	message string
}

// WithMessage sets the error message.
func WithMessage(msg string) FormatOption {
	return FormatOption{message: msg}
}

// ValidateLength validates string length.
func (cs *Changeset) ValidateLength(field string, opts LengthOpts) *Changeset {
	value := cs.GetString(field)
	length := len([]rune(value))

	if opts.Min > 0 && length < opts.Min {
		cs.AddError(field, fmt.Sprintf("should be at least %d character(s)", opts.Min))
	}

	if opts.Max > 0 && length > opts.Max {
		cs.AddError(field, fmt.Sprintf("should be at most %d character(s)", opts.Max))
	}

	if opts.Is > 0 && length != opts.Is {
		cs.AddError(field, fmt.Sprintf("should be %d character(s)", opts.Is))
	}

	return cs
}

// LengthOpts configures length validation.
type LengthOpts struct {
	Min int
	Max int
	Is  int
}

// ValidateNumber validates a numeric field.
func (cs *Changeset) ValidateNumber(field string, opts NumberOpts) *Changeset {
	value := toFloat64(cs.GetField(field))

	if opts.GreaterThan != nil && value <= *opts.GreaterThan {
		cs.AddError(field, fmt.Sprintf("must be greater than %v", *opts.GreaterThan))
	}

	if opts.GreaterThanOrEq != nil && value < *opts.GreaterThanOrEq {
		cs.AddError(field, fmt.Sprintf("must be greater than or equal to %v", *opts.GreaterThanOrEq))
	}

	if opts.LessThan != nil && value >= *opts.LessThan {
		cs.AddError(field, fmt.Sprintf("must be less than %v", *opts.LessThan))
	}

	if opts.LessThanOrEq != nil && value > *opts.LessThanOrEq {
		cs.AddError(field, fmt.Sprintf("must be less than or equal to %v", *opts.LessThanOrEq))
	}

	return cs
}

// NumberOpts configures number validation.
type NumberOpts struct {
	GreaterThan     *float64
	GreaterThanOrEq *float64
	LessThan        *float64
	LessThanOrEq    *float64
}

// ValidateInclusion validates value is in a list.
func (cs *Changeset) ValidateInclusion(field string, values []any) *Changeset {
	value := cs.GetField(field)
	for _, v := range values {
		if value == v {
			return cs
		}
	}
	cs.AddError(field, "is invalid")
	return cs
}

// ValidateExclusion validates value is not in a list.
func (cs *Changeset) ValidateExclusion(field string, values []any) *Changeset {
	value := cs.GetField(field)
	for _, v := range values {
		if value == v {
			cs.AddError(field, "is reserved")
			return cs
		}
	}
	return cs
}

// ValidateConfirmation validates two fields match.
func (cs *Changeset) ValidateConfirmation(field string) *Changeset {
	value := cs.GetField(field)
	confirmation := cs.GetField(field + "_confirmation")

	if value != confirmation {
		cs.AddError(field + "_confirmation", "does not match")
	}

	return cs
}

// ValidateCustom runs a custom validation function.
func (cs *Changeset) ValidateCustom(fn func(*Changeset) *Changeset) *Changeset {
	return fn(cs)
}

// AddError adds an error to a field.
func (cs *Changeset) AddError(field, message string) *Changeset {
	cs.Errors[field] = append(cs.Errors[field], message)
	cs.Valid = false
	return cs
}

// HasError returns true if a field has errors.
func (cs *Changeset) HasError(field string) bool {
	return len(cs.Errors[field]) > 0
}

// FieldErrors returns errors for a specific field.
func (cs *Changeset) FieldErrors(field string) []string {
	return cs.Errors[field]
}

// FirstError returns the first error for a field.
func (cs *Changeset) FirstError(field string) string {
	if errs := cs.Errors[field]; len(errs) > 0 {
		return errs[0]
	}
	return ""
}

// ErrorMessages returns all errors as a single string.
func (cs *Changeset) ErrorMessages() string {
	var msgs []string
	for field, errs := range cs.Errors {
		for _, err := range errs {
			msgs = append(msgs, fmt.Sprintf("%s %s", field, err))
		}
	}
	return strings.Join(msgs, ", ")
}

// Apply returns the merged data with changes.
// Returns error if changeset is invalid.
func (cs *Changeset) Apply() (map[string]any, error) {
	if !cs.Valid {
		return nil, fmt.Errorf("changeset is invalid: %s", cs.ErrorMessages())
	}

	result := make(map[string]any)
	for k, v := range cs.Data {
		result[k] = v
	}
	for k, v := range cs.Changes {
		result[k] = v
	}

	return result, nil
}

// ApplyChanges returns changes only (for partial updates).
func (cs *Changeset) ApplyChanges() (map[string]any, error) {
	if !cs.Valid {
		return nil, fmt.Errorf("changeset is invalid: %s", cs.ErrorMessages())
	}

	result := make(map[string]any)
	for k, v := range cs.Changes {
		result[k] = v
	}

	return result, nil
}

// HasChanges returns true if there are any changes.
func (cs *Changeset) HasChanges() bool {
	return len(cs.Changes) > 0
}

// SetAction sets the changeset action.
func (cs *Changeset) SetAction(action string) *Changeset {
	cs.Action = action
	return cs
}

// PutChange adds or updates a change.
func (cs *Changeset) PutChange(key string, value any) *Changeset {
	cs.Changes[key] = value
	return cs
}

// DeleteChange removes a change.
func (cs *Changeset) DeleteChange(key string) *Changeset {
	delete(cs.Changes, key)
	return cs
}

// ForceChange adds a change even if the value hasn't changed.
func (cs *Changeset) ForceChange(key string, value any) *Changeset {
	cs.Changes[key] = value
	return cs
}
