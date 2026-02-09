// Package forms provides form handling for GoliveKit applications.
package forms

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Form represents a form with fields, data, and validation state.
type Form struct {
	// Name is the form identifier.
	Name string

	// Fields are the form fields.
	Fields []Field

	// Data contains the current form values.
	Data map[string]any

	// Errors contains validation errors keyed by field name.
	Errors map[string][]string

	// Submitting indicates if the form is being submitted.
	Submitting bool

	// Valid indicates if the form has passed validation.
	Valid bool

	// CSRF is the CSRF token for the form.
	CSRF string

	// Action is the form action URL.
	Action string

	// Method is the HTTP method (default POST).
	Method string

	mu sync.RWMutex
}

// NewForm creates a new form.
func NewForm(name string) *Form {
	return &Form{
		Name:   name,
		Fields: make([]Field, 0),
		Data:   make(map[string]any),
		Errors: make(map[string][]string),
		Method: "POST",
	}
}

// AddField adds a field to the form.
func (f *Form) AddField(field Field) *Form {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Fields = append(f.Fields, field)
	return f
}

// AddFields adds multiple fields to the form.
func (f *Form) AddFields(fields ...Field) *Form {
	for _, field := range fields {
		f.AddField(field)
	}
	return f
}

// GetField retrieves a field by name.
func (f *Form) GetField(name string) *Field {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for i := range f.Fields {
		if f.Fields[i].Name == name {
			return &f.Fields[i]
		}
	}
	return nil
}

// SetValue sets a field value.
func (f *Form) SetValue(name string, value any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Data[name] = value
}

// GetValue retrieves a field value.
func (f *Form) GetValue(name string) any {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.Data[name]
}

// GetString retrieves a string value.
func (f *Form) GetString(name string) string {
	if v, ok := f.GetValue(name).(string); ok {
		return v
	}
	return ""
}

// GetInt retrieves an int value.
func (f *Form) GetInt(name string) int {
	switch v := f.GetValue(name).(type) {
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

// GetBool retrieves a bool value.
func (f *Form) GetBool(name string) bool {
	if v, ok := f.GetValue(name).(bool); ok {
		return v
	}
	return false
}

// AddError adds an error for a field.
func (f *Form) AddError(field, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Errors[field] = append(f.Errors[field], message)
	f.Valid = false
}

// HasErrors returns true if the form has any errors.
func (f *Form) HasErrors() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.Errors) > 0
}

// FieldErrors returns errors for a specific field.
func (f *Form) FieldErrors(name string) []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.Errors[name]
}

// ClearErrors clears all errors.
func (f *Form) ClearErrors() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Errors = make(map[string][]string)
	f.Valid = true
}

// Bind binds data from an HTTP request.
func (f *Form) Bind(r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	return f.BindValues(r.Form)
}

// BindValues binds data from url.Values.
func (f *Form) BindValues(values url.Values) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, field := range f.Fields {
		if values.Has(field.Name) {
			value := values.Get(field.Name)
			f.Data[field.Name] = convertValue(value, field.Type)
		}
	}

	return nil
}

// BindJSON binds data from JSON.
func (f *Form) BindJSON(data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return json.Unmarshal(data, &f.Data)
}

// BindMap binds data from a map.
func (f *Form) BindMap(data map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for key, value := range data {
		f.Data[key] = value
	}
}

// Validate validates all fields.
func (f *Form) Validate() bool {
	f.ClearErrors()

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, field := range f.Fields {
		value := f.Data[field.Name]

		// Check required
		if field.Required && isEmpty(value) {
			f.Errors[field.Name] = append(f.Errors[field.Name], "This field is required")
			continue
		}

		// Run validators
		for _, validator := range field.Validators {
			if err := validator.Validate(value); err != nil {
				f.Errors[field.Name] = append(f.Errors[field.Name], validator.Message())
			}
		}
	}

	f.Valid = len(f.Errors) == 0
	return f.Valid
}

// ToJSON converts the form data to JSON.
func (f *Form) ToJSON() ([]byte, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return json.Marshal(f.Data)
}

// Reset clears all data and errors.
func (f *Form) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Data = make(map[string]any)
	f.Errors = make(map[string][]string)
	f.Submitting = false
	f.Valid = false
}

// convertValue converts a string value to the appropriate type.
func convertValue(value string, fieldType FieldType) any {
	switch fieldType {
	case FieldNumber:
		var n float64
		if _, err := parseFloat(value); err == nil {
			return n
		}
		return 0
	case FieldCheckbox:
		return value == "true" || value == "on" || value == "1"
	default:
		return value
	}
}

func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	var f float64
	err := json.Unmarshal([]byte(s), &f)
	return f, err
}

func isEmpty(value any) bool {
	if value == nil {
		return true
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	default:
		return false
	}
}

// FormBuilder provides a fluent API for building forms.
type FormBuilder struct {
	form *Form
}

// NewFormBuilder creates a new form builder.
func NewFormBuilder(name string) *FormBuilder {
	return &FormBuilder{form: NewForm(name)}
}

// Action sets the form action.
func (fb *FormBuilder) Action(action string) *FormBuilder {
	fb.form.Action = action
	return fb
}

// Method sets the form method.
func (fb *FormBuilder) Method(method string) *FormBuilder {
	fb.form.Method = method
	return fb
}

// CSRF sets the CSRF token.
func (fb *FormBuilder) CSRF(token string) *FormBuilder {
	fb.form.CSRF = token
	return fb
}

// Text adds a text field.
func (fb *FormBuilder) Text(name, label string, opts ...FieldOption) *FormBuilder {
	field := NewField(name, FieldText, label, opts...)
	fb.form.AddField(field)
	return fb
}

// Email adds an email field.
func (fb *FormBuilder) Email(name, label string, opts ...FieldOption) *FormBuilder {
	field := NewField(name, FieldEmail, label, opts...)
	field.Validators = append(field.Validators, EmailValidator{})
	fb.form.AddField(field)
	return fb
}

// Password adds a password field.
func (fb *FormBuilder) Password(name, label string, opts ...FieldOption) *FormBuilder {
	field := NewField(name, FieldPassword, label, opts...)
	fb.form.AddField(field)
	return fb
}

// Number adds a number field.
func (fb *FormBuilder) Number(name, label string, opts ...FieldOption) *FormBuilder {
	field := NewField(name, FieldNumber, label, opts...)
	fb.form.AddField(field)
	return fb
}

// Textarea adds a textarea field.
func (fb *FormBuilder) Textarea(name, label string, opts ...FieldOption) *FormBuilder {
	field := NewField(name, FieldTextarea, label, opts...)
	fb.form.AddField(field)
	return fb
}

// Select adds a select field.
func (fb *FormBuilder) Select(name, label string, options []Option, opts ...FieldOption) *FormBuilder {
	field := NewField(name, FieldSelect, label, opts...)
	field.Options = options
	fb.form.AddField(field)
	return fb
}

// Checkbox adds a checkbox field.
func (fb *FormBuilder) Checkbox(name, label string, opts ...FieldOption) *FormBuilder {
	field := NewField(name, FieldCheckbox, label, opts...)
	fb.form.AddField(field)
	return fb
}

// Hidden adds a hidden field.
func (fb *FormBuilder) Hidden(name string, opts ...FieldOption) *FormBuilder {
	field := NewField(name, FieldHidden, "", opts...)
	fb.form.AddField(field)
	return fb
}

// Date adds a date field.
func (fb *FormBuilder) Date(name, label string, opts ...FieldOption) *FormBuilder {
	field := NewField(name, FieldDate, label, opts...)
	fb.form.AddField(field)
	return fb
}

// Build returns the constructed form.
func (fb *FormBuilder) Build() *Form {
	return fb.form
}
