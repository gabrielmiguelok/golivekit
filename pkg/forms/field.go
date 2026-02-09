package forms

// FieldType identifies the type of form field.
type FieldType string

const (
	FieldText     FieldType = "text"
	FieldEmail    FieldType = "email"
	FieldPassword FieldType = "password"
	FieldNumber   FieldType = "number"
	FieldTextarea FieldType = "textarea"
	FieldSelect   FieldType = "select"
	FieldCheckbox FieldType = "checkbox"
	FieldRadio    FieldType = "radio"
	FieldFile     FieldType = "file"
	FieldHidden   FieldType = "hidden"
	FieldDate     FieldType = "date"
	FieldTime     FieldType = "time"
	FieldDateTime FieldType = "datetime-local"
	FieldURL      FieldType = "url"
	FieldTel      FieldType = "tel"
	FieldColor    FieldType = "color"
	FieldRange    FieldType = "range"
)

// Field represents a form field.
type Field struct {
	// Name is the field name (used in form data).
	Name string

	// Type is the field type.
	Type FieldType

	// Label is the display label.
	Label string

	// Placeholder is the placeholder text.
	Placeholder string

	// Required indicates if the field is required.
	Required bool

	// Disabled indicates if the field is disabled.
	Disabled bool

	// ReadOnly indicates if the field is read-only.
	ReadOnly bool

	// Validators are the field validators.
	Validators []Validator

	// Options are the available options (for select/radio fields).
	Options []Option

	// Value is the current field value.
	Value any

	// Default is the default value.
	Default any

	// Error is the current error message.
	Error string

	// Help is help text shown below the field.
	Help string

	// Attrs are additional HTML attributes.
	Attrs map[string]string

	// Class is the CSS class(es) for the field.
	Class string

	// Min/Max for number/date fields.
	Min any
	Max any

	// Step for number fields.
	Step any

	// Pattern for regex validation (HTML5).
	Pattern string

	// Multiple allows multiple values (for select/file).
	Multiple bool

	// Autocomplete attribute.
	Autocomplete string
}

// Option represents a select/radio option.
type Option struct {
	Value    string
	Label    string
	Selected bool
	Disabled bool
	Group    string // For option groups
}

// FieldOption is a function that configures a field.
type FieldOption func(*Field)

// NewField creates a new field.
func NewField(name string, fieldType FieldType, label string, opts ...FieldOption) Field {
	field := Field{
		Name:       name,
		Type:       fieldType,
		Label:      label,
		Validators: make([]Validator, 0),
		Options:    make([]Option, 0),
		Attrs:      make(map[string]string),
	}

	for _, opt := range opts {
		opt(&field)
	}

	return field
}

// Field options

// WithRequired marks the field as required.
func WithRequired() FieldOption {
	return func(f *Field) {
		f.Required = true
	}
}

// WithPlaceholder sets the placeholder text.
func WithPlaceholder(placeholder string) FieldOption {
	return func(f *Field) {
		f.Placeholder = placeholder
	}
}

// WithDefault sets the default value.
func WithDefault(value any) FieldOption {
	return func(f *Field) {
		f.Default = value
		f.Value = value
	}
}

// WithValue sets the current value.
func WithValue(value any) FieldOption {
	return func(f *Field) {
		f.Value = value
	}
}

// WithHelp sets the help text.
func WithHelp(help string) FieldOption {
	return func(f *Field) {
		f.Help = help
	}
}

// WithClass sets the CSS class.
func WithClass(class string) FieldOption {
	return func(f *Field) {
		f.Class = class
	}
}

// WithAttr sets an HTML attribute.
func WithAttr(key, value string) FieldOption {
	return func(f *Field) {
		f.Attrs[key] = value
	}
}

// WithDisabled marks the field as disabled.
func WithDisabled() FieldOption {
	return func(f *Field) {
		f.Disabled = true
	}
}

// WithReadOnly marks the field as read-only.
func WithReadOnly() FieldOption {
	return func(f *Field) {
		f.ReadOnly = true
	}
}

// WithValidator adds a validator.
func WithValidator(v Validator) FieldOption {
	return func(f *Field) {
		f.Validators = append(f.Validators, v)
	}
}

// WithMinLength adds a minimum length validator.
func WithMinLength(n int) FieldOption {
	return func(f *Field) {
		f.Validators = append(f.Validators, MinLengthValidator{Min: n})
	}
}

// WithMaxLength adds a maximum length validator.
func WithMaxLength(n int) FieldOption {
	return func(f *Field) {
		f.Validators = append(f.Validators, MaxLengthValidator{Max: n})
	}
}

// WithMin sets the minimum value.
func WithMin(min any) FieldOption {
	return func(f *Field) {
		f.Min = min
	}
}

// WithMax sets the maximum value.
func WithMax(max any) FieldOption {
	return func(f *Field) {
		f.Max = max
	}
}

// WithPattern sets the regex pattern.
func WithPattern(pattern string) FieldOption {
	return func(f *Field) {
		f.Pattern = pattern
		f.Validators = append(f.Validators, PatternValidator{Pattern: pattern})
	}
}

// WithOptions sets the select/radio options.
func WithOptions(options ...Option) FieldOption {
	return func(f *Field) {
		f.Options = options
	}
}

// WithMultiple allows multiple values.
func WithMultiple() FieldOption {
	return func(f *Field) {
		f.Multiple = true
	}
}

// WithAutocomplete sets the autocomplete attribute.
func WithAutocomplete(value string) FieldOption {
	return func(f *Field) {
		f.Autocomplete = value
	}
}

// TextField creates a text field.
func TextField(name, label string, opts ...FieldOption) Field {
	return NewField(name, FieldText, label, opts...)
}

// EmailField creates an email field.
func EmailField(name, label string, opts ...FieldOption) Field {
	field := NewField(name, FieldEmail, label, opts...)
	field.Validators = append(field.Validators, EmailValidator{})
	return field
}

// PasswordField creates a password field.
func PasswordField(name, label string, opts ...FieldOption) Field {
	return NewField(name, FieldPassword, label, opts...)
}

// NumberField creates a number field.
func NumberField(name, label string, opts ...FieldOption) Field {
	return NewField(name, FieldNumber, label, opts...)
}

// TextareaField creates a textarea field.
func TextareaField(name, label string, opts ...FieldOption) Field {
	return NewField(name, FieldTextarea, label, opts...)
}

// SelectField creates a select field.
func SelectField(name, label string, options []Option, opts ...FieldOption) Field {
	field := NewField(name, FieldSelect, label, opts...)
	field.Options = options
	return field
}

// CheckboxField creates a checkbox field.
func CheckboxField(name, label string, opts ...FieldOption) Field {
	return NewField(name, FieldCheckbox, label, opts...)
}

// RadioField creates a radio field.
func RadioField(name, label string, options []Option, opts ...FieldOption) Field {
	field := NewField(name, FieldRadio, label, opts...)
	field.Options = options
	return field
}

// HiddenField creates a hidden field.
func HiddenField(name string, opts ...FieldOption) Field {
	return NewField(name, FieldHidden, "", opts...)
}

// DateField creates a date field.
func DateField(name, label string, opts ...FieldOption) Field {
	return NewField(name, FieldDate, label, opts...)
}

// FileField creates a file upload field.
func FileField(name, label string, opts ...FieldOption) Field {
	return NewField(name, FieldFile, label, opts...)
}

// URLField creates a URL field.
func URLField(name, label string, opts ...FieldOption) Field {
	field := NewField(name, FieldURL, label, opts...)
	field.Validators = append(field.Validators, URLValidator{})
	return field
}
