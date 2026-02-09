package forms

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Validator validates a field value.
type Validator interface {
	// Validate checks if the value is valid.
	Validate(value any) error

	// Message returns the error message.
	Message() string
}

// RequiredValidator validates that a field is not empty.
type RequiredValidator struct{}

func (v RequiredValidator) Validate(value any) error {
	if value == nil {
		return errors.New("required")
	}
	switch val := value.(type) {
	case string:
		if strings.TrimSpace(val) == "" {
			return errors.New("required")
		}
	case []any:
		if len(val) == 0 {
			return errors.New("required")
		}
	}
	return nil
}

func (v RequiredValidator) Message() string {
	return "This field is required"
}

// EmailValidator validates email format.
type EmailValidator struct{}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

func (v EmailValidator) Validate(value any) error {
	str, ok := value.(string)
	if !ok || str == "" {
		return nil // Skip if empty (use Required for that)
	}
	if !emailRegex.MatchString(str) {
		return errors.New("invalid email")
	}
	return nil
}

func (v EmailValidator) Message() string {
	return "Please enter a valid email address"
}

// URLValidator validates URL format.
type URLValidator struct{}

var urlRegex = regexp.MustCompile(`^https?://[a-zA-Z0-9.-]+(:\d+)?(/.*)?$`)

func (v URLValidator) Validate(value any) error {
	str, ok := value.(string)
	if !ok || str == "" {
		return nil
	}
	if !urlRegex.MatchString(str) {
		return errors.New("invalid URL")
	}
	return nil
}

func (v URLValidator) Message() string {
	return "Please enter a valid URL"
}

// MinLengthValidator validates minimum string length.
type MinLengthValidator struct {
	Min int
}

func (v MinLengthValidator) Validate(value any) error {
	str, ok := value.(string)
	if !ok || str == "" {
		return nil
	}
	if utf8.RuneCountInString(str) < v.Min {
		return fmt.Errorf("too short (min %d)", v.Min)
	}
	return nil
}

func (v MinLengthValidator) Message() string {
	return fmt.Sprintf("Must be at least %d characters", v.Min)
}

// MaxLengthValidator validates maximum string length.
type MaxLengthValidator struct {
	Max int
}

func (v MaxLengthValidator) Validate(value any) error {
	str, ok := value.(string)
	if !ok {
		return nil
	}
	if utf8.RuneCountInString(str) > v.Max {
		return fmt.Errorf("too long (max %d)", v.Max)
	}
	return nil
}

func (v MaxLengthValidator) Message() string {
	return fmt.Sprintf("Must be at most %d characters", v.Max)
}

// PatternValidator validates against a regex pattern.
type PatternValidator struct {
	Pattern string
	Msg     string
}

func (v PatternValidator) Validate(value any) error {
	str, ok := value.(string)
	if !ok || str == "" {
		return nil
	}
	matched, err := regexp.MatchString(v.Pattern, str)
	if err != nil {
		return err
	}
	if !matched {
		return errors.New("pattern mismatch")
	}
	return nil
}

func (v PatternValidator) Message() string {
	if v.Msg != "" {
		return v.Msg
	}
	return "Invalid format"
}

// MinValidator validates minimum numeric value.
type MinValidator struct {
	Min float64
}

func (v MinValidator) Validate(value any) error {
	num := toFloat64(value)
	if num < v.Min {
		return fmt.Errorf("must be at least %v", v.Min)
	}
	return nil
}

func (v MinValidator) Message() string {
	return fmt.Sprintf("Must be at least %v", v.Min)
}

// MaxValidator validates maximum numeric value.
type MaxValidator struct {
	Max float64
}

func (v MaxValidator) Validate(value any) error {
	num := toFloat64(value)
	if num > v.Max {
		return fmt.Errorf("must be at most %v", v.Max)
	}
	return nil
}

func (v MaxValidator) Message() string {
	return fmt.Sprintf("Must be at most %v", v.Max)
}

// RangeValidator validates a numeric range.
type RangeValidator struct {
	Min float64
	Max float64
}

func (v RangeValidator) Validate(value any) error {
	num := toFloat64(value)
	if num < v.Min || num > v.Max {
		return fmt.Errorf("must be between %v and %v", v.Min, v.Max)
	}
	return nil
}

func (v RangeValidator) Message() string {
	return fmt.Sprintf("Must be between %v and %v", v.Min, v.Max)
}

// OneOfValidator validates that value is one of allowed values.
type OneOfValidator struct {
	Values []any
}

func (v OneOfValidator) Validate(value any) error {
	for _, allowed := range v.Values {
		if value == allowed {
			return nil
		}
	}
	return errors.New("invalid option")
}

func (v OneOfValidator) Message() string {
	return "Invalid selection"
}

// CustomValidator allows custom validation functions.
type CustomValidator struct {
	Fn  func(value any) error
	Msg string
}

func (v CustomValidator) Validate(value any) error {
	return v.Fn(value)
}

func (v CustomValidator) Message() string {
	return v.Msg
}

// Helper functions

func toFloat64(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	default:
		return 0
	}
}

// Convenience constructors

// Required returns a required validator.
func Required() Validator {
	return RequiredValidator{}
}

// Email returns an email validator.
func Email() Validator {
	return EmailValidator{}
}

// URL returns a URL validator.
func URL() Validator {
	return URLValidator{}
}

// MinLength returns a minimum length validator.
func MinLength(n int) Validator {
	return MinLengthValidator{Min: n}
}

// MaxLength returns a maximum length validator.
func MaxLength(n int) Validator {
	return MaxLengthValidator{Max: n}
}

// Pattern returns a pattern validator.
func Pattern(pattern string, msg ...string) Validator {
	v := PatternValidator{Pattern: pattern}
	if len(msg) > 0 {
		v.Msg = msg[0]
	}
	return v
}

// Min returns a minimum value validator.
func Min(n float64) Validator {
	return MinValidator{Min: n}
}

// Max returns a maximum value validator.
func Max(n float64) Validator {
	return MaxValidator{Max: n}
}

// Range returns a range validator.
func Range(min, max float64) Validator {
	return RangeValidator{Min: min, Max: max}
}

// OneOf returns a one-of validator.
func OneOf(values ...any) Validator {
	return OneOfValidator{Values: values}
}

// Custom returns a custom validator.
func Custom(fn func(value any) error, msg string) Validator {
	return CustomValidator{Fn: fn, Msg: msg}
}
