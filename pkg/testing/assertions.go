package testing

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// Assert provides assertion helpers for tests.
type Assert struct {
	t *testing.T
}

// NewAssert creates a new Assert instance.
func NewAssert(t *testing.T) *Assert {
	return &Assert{t: t}
}

// True asserts that a condition is true.
func (a *Assert) True(condition bool, msgAndArgs ...any) {
	a.t.Helper()
	if !condition {
		a.fail("Expected true but got false", msgAndArgs...)
	}
}

// False asserts that a condition is false.
func (a *Assert) False(condition bool, msgAndArgs ...any) {
	a.t.Helper()
	if condition {
		a.fail("Expected false but got true", msgAndArgs...)
	}
}

// Equal asserts that two values are equal.
func (a *Assert) Equal(expected, actual any, msgAndArgs ...any) {
	a.t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		a.fail(fmt.Sprintf("Expected %v (%T) but got %v (%T)", expected, expected, actual, actual), msgAndArgs...)
	}
}

// NotEqual asserts that two values are not equal.
func (a *Assert) NotEqual(expected, actual any, msgAndArgs ...any) {
	a.t.Helper()
	if reflect.DeepEqual(expected, actual) {
		a.fail(fmt.Sprintf("Expected values to be different but both were %v", actual), msgAndArgs...)
	}
}

// Nil asserts that a value is nil.
func (a *Assert) Nil(value any, msgAndArgs ...any) {
	a.t.Helper()
	if !isNil(value) {
		a.fail(fmt.Sprintf("Expected nil but got %v", value), msgAndArgs...)
	}
}

// NotNil asserts that a value is not nil.
func (a *Assert) NotNil(value any, msgAndArgs ...any) {
	a.t.Helper()
	if isNil(value) {
		a.fail("Expected non-nil value but got nil", msgAndArgs...)
	}
}

// NoError asserts that an error is nil.
func (a *Assert) NoError(err error, msgAndArgs ...any) {
	a.t.Helper()
	if err != nil {
		a.fail(fmt.Sprintf("Expected no error but got: %v", err), msgAndArgs...)
	}
}

// Error asserts that an error is not nil.
func (a *Assert) Error(err error, msgAndArgs ...any) {
	a.t.Helper()
	if err == nil {
		a.fail("Expected an error but got nil", msgAndArgs...)
	}
}

// ErrorContains asserts that an error message contains a substring.
func (a *Assert) ErrorContains(err error, substring string, msgAndArgs ...any) {
	a.t.Helper()
	if err == nil {
		a.fail(fmt.Sprintf("Expected error containing %q but got nil", substring), msgAndArgs...)
		return
	}
	if !strings.Contains(err.Error(), substring) {
		a.fail(fmt.Sprintf("Expected error containing %q but got %q", substring, err.Error()), msgAndArgs...)
	}
}

// Contains asserts that a string contains a substring.
func (a *Assert) Contains(str, substring string, msgAndArgs ...any) {
	a.t.Helper()
	if !strings.Contains(str, substring) {
		a.fail(fmt.Sprintf("Expected %q to contain %q", str, substring), msgAndArgs...)
	}
}

// NotContains asserts that a string does not contain a substring.
func (a *Assert) NotContains(str, substring string, msgAndArgs ...any) {
	a.t.Helper()
	if strings.Contains(str, substring) {
		a.fail(fmt.Sprintf("Expected %q to not contain %q", str, substring), msgAndArgs...)
	}
}

// Matches asserts that a string matches a regex pattern.
func (a *Assert) Matches(pattern, str string, msgAndArgs ...any) {
	a.t.Helper()
	matched, err := regexp.MatchString(pattern, str)
	if err != nil {
		a.fail(fmt.Sprintf("Invalid regex pattern: %v", err), msgAndArgs...)
		return
	}
	if !matched {
		a.fail(fmt.Sprintf("Expected %q to match pattern %q", str, pattern), msgAndArgs...)
	}
}

// Len asserts that a collection has a specific length.
func (a *Assert) Len(collection any, length int, msgAndArgs ...any) {
	a.t.Helper()
	actual := reflect.ValueOf(collection).Len()
	if actual != length {
		a.fail(fmt.Sprintf("Expected length %d but got %d", length, actual), msgAndArgs...)
	}
}

// Empty asserts that a collection is empty.
func (a *Assert) Empty(collection any, msgAndArgs ...any) {
	a.t.Helper()
	if reflect.ValueOf(collection).Len() != 0 {
		a.fail(fmt.Sprintf("Expected empty collection but got %v", collection), msgAndArgs...)
	}
}

// NotEmpty asserts that a collection is not empty.
func (a *Assert) NotEmpty(collection any, msgAndArgs ...any) {
	a.t.Helper()
	if reflect.ValueOf(collection).Len() == 0 {
		a.fail("Expected non-empty collection but got empty", msgAndArgs...)
	}
}

// Panics asserts that a function panics.
func (a *Assert) Panics(fn func(), msgAndArgs ...any) {
	a.t.Helper()
	defer func() {
		if r := recover(); r == nil {
			a.fail("Expected panic but function did not panic", msgAndArgs...)
		}
	}()
	fn()
}

// NotPanics asserts that a function does not panic.
func (a *Assert) NotPanics(fn func(), msgAndArgs ...any) {
	a.t.Helper()
	defer func() {
		if r := recover(); r != nil {
			a.fail(fmt.Sprintf("Expected no panic but got: %v", r), msgAndArgs...)
		}
	}()
	fn()
}

// Eventually asserts that a condition becomes true within a timeout.
func (a *Assert) Eventually(condition func() bool, maxRetries int, msgAndArgs ...any) {
	a.t.Helper()
	for i := 0; i < maxRetries; i++ {
		if condition() {
			return
		}
	}
	a.fail("Condition never became true", msgAndArgs...)
}

// fail records a test failure.
func (a *Assert) fail(message string, msgAndArgs ...any) {
	a.t.Helper()
	if len(msgAndArgs) > 0 {
		message = fmt.Sprintf("%s: %s", message, fmt.Sprint(msgAndArgs...))
	}
	a.t.Error(message)
}

// isNil checks if a value is nil.
func isNil(value any) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	}

	return false
}

// HTMLAssert provides HTML-specific assertions.
type HTMLAssert struct {
	*Assert
	html string
}

// NewHTMLAssert creates a new HTML assertion helper.
func NewHTMLAssert(t *testing.T, html string) *HTMLAssert {
	return &HTMLAssert{
		Assert: NewAssert(t),
		html:   html,
	}
}

// HasElement asserts that the HTML contains an element matching a pattern.
func (ha *HTMLAssert) HasElement(tag string, attrs ...string) {
	ha.t.Helper()

	pattern := fmt.Sprintf("<%s", tag)
	if !strings.Contains(ha.html, pattern) {
		ha.fail(fmt.Sprintf("Element <%s> not found in HTML", tag))
		return
	}

	for _, attr := range attrs {
		if !strings.Contains(ha.html, attr) {
			ha.fail(fmt.Sprintf("Attribute %q not found in HTML", attr))
		}
	}
}

// HasText asserts that the HTML contains specific text.
func (ha *HTMLAssert) HasText(text string) {
	ha.t.Helper()
	ha.Contains(ha.html, text)
}

// HasClass asserts that the HTML contains an element with a specific class.
func (ha *HTMLAssert) HasClass(class string) {
	ha.t.Helper()
	pattern := fmt.Sprintf(`class="[^"]*%s[^"]*"`, class)
	ha.Matches(pattern, ha.html)
}

// HasID asserts that the HTML contains an element with a specific ID.
func (ha *HTMLAssert) HasID(id string) {
	ha.t.Helper()
	pattern := fmt.Sprintf(`id="%s"`, id)
	ha.Contains(ha.html, pattern)
}
