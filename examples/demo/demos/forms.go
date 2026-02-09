// Package demos provides demo components for GoliveKit showcase.
package demos

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/gabrielmiguelok/golivekit/internal/website"
	"github.com/gabrielmiguelok/golivekit/internal/website/components"
	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// WizardStep represents a step in the wizard
type WizardStep int

const (
	StepBasics WizardStep = iota
	StepProfile
	StepPreferences
	StepReview
)

// FieldError represents a validation error
type FieldError struct {
	Field   string
	Message string
}

// FormsWizard is the multi-step form wizard component.
type FormsWizard struct {
	core.BaseComponent

	// Current step
	CurrentStep WizardStep

	// Step 1: Basics
	Email           string
	EmailChecking   bool
	EmailAvailable  bool
	Password        string
	PasswordConfirm string
	PasswordStrength int // 0-4

	// Step 2: Profile
	FullName         string
	Username         string
	UsernameChecking bool
	UsernameAvailable bool
	Bio              string
	AvatarFilename   string
	AvatarProgress   int

	// Step 3: Preferences
	Theme         string // "light", "dark", "system"
	Notifications struct {
		Email    bool
		Push     bool
		SMS      bool
		Weekly   bool
	}
	Language string

	// Validation state
	Errors       []FieldError
	StepComplete [4]bool

	// CSRF token (simulated)
	CSRFToken string

	// Submission state
	Submitted     bool
	SubmitSuccess bool
}

// Common username patterns that are taken
var takenUsernames = map[string]bool{
	"admin": true, "root": true, "user": true, "test": true,
	"johndoe": true, "janedoe": true, "alice": true, "bob": true,
}

// Taken emails for simulation
var takenEmails = map[string]bool{
	"test@example.com": true, "admin@example.com": true,
}

// NewFormsWizard creates a new forms wizard component.
func NewFormsWizard() core.Component {
	return &FormsWizard{}
}

// Name returns the component name.
func (f *FormsWizard) Name() string {
	return "forms-wizard"
}

// Mount initializes the wizard.
func (f *FormsWizard) Mount(ctx context.Context, params core.Params, session core.Session) error {
	f.CurrentStep = StepBasics
	f.Theme = "system"
	f.Language = "en"
	f.Notifications.Email = true
	f.Notifications.Weekly = true
	f.CSRFToken = fmt.Sprintf("csrf_%d", time.Now().UnixNano())
	return nil
}

// Terminate handles cleanup.
func (f *FormsWizard) Terminate(ctx context.Context, reason core.TerminateReason) error {
	return nil
}

// HandleEvent handles user interactions.
func (f *FormsWizard) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	switch event {
	// Navigation
	case "next_step":
		if f.validateCurrentStep() {
			if f.CurrentStep < StepReview {
				f.CurrentStep++
			}
		}

	case "prev_step":
		if f.CurrentStep > StepBasics {
			f.CurrentStep--
		}

	case "goto_step":
		if step, ok := payload["step"].(float64); ok {
			targetStep := WizardStep(int(step))
			// Can only go to completed steps or current+1
			if targetStep <= f.CurrentStep || f.StepComplete[targetStep-1] {
				f.CurrentStep = targetStep
			}
		}

	// Step 1: Basics
	case "update_email":
		if val, ok := payload["value"].(string); ok {
			f.Email = strings.TrimSpace(val)
			f.EmailChecking = true
			f.removeError("email")
		}

	case "check_email":
		f.EmailChecking = false
		f.EmailAvailable = !takenEmails[strings.ToLower(f.Email)]
		if !f.EmailAvailable {
			f.addError("email", "This email is already registered")
		}

	case "update_password":
		if val, ok := payload["value"].(string); ok {
			f.Password = val
			f.PasswordStrength = f.calculatePasswordStrength(val)
			f.removeError("password")
		}

	case "update_password_confirm":
		if val, ok := payload["value"].(string); ok {
			f.PasswordConfirm = val
			f.removeError("password_confirm")
		}

	// Step 2: Profile
	case "update_fullname":
		if val, ok := payload["value"].(string); ok {
			f.FullName = val
			f.removeError("fullname")
		}

	case "update_username":
		if val, ok := payload["value"].(string); ok {
			f.Username = strings.ToLower(strings.TrimSpace(val))
			f.UsernameChecking = true
			f.removeError("username")
		}

	case "check_username":
		f.UsernameChecking = false
		f.UsernameAvailable = !takenUsernames[f.Username]
		if !f.UsernameAvailable {
			f.addError("username", "This username is already taken")
		}

	case "update_bio":
		if val, ok := payload["value"].(string); ok {
			if len(val) <= 500 {
				f.Bio = val
			}
		}

	case "upload_avatar":
		// Simulate file upload progress
		if filename, ok := payload["filename"].(string); ok {
			f.AvatarFilename = filename
			f.AvatarProgress = 0
		}

	case "avatar_progress":
		if progress, ok := payload["progress"].(float64); ok {
			f.AvatarProgress = int(progress)
		}

	// Step 3: Preferences
	case "update_theme":
		if val, ok := payload["value"].(string); ok {
			f.Theme = val
		}

	case "update_language":
		if val, ok := payload["value"].(string); ok {
			f.Language = val
		}

	case "toggle_notification":
		if notifType, ok := payload["type"].(string); ok {
			switch notifType {
			case "email":
				f.Notifications.Email = !f.Notifications.Email
			case "push":
				f.Notifications.Push = !f.Notifications.Push
			case "sms":
				f.Notifications.SMS = !f.Notifications.SMS
			case "weekly":
				f.Notifications.Weekly = !f.Notifications.Weekly
			}
		}

	// Step 4: Submit
	case "submit":
		if f.validateAllSteps() {
			f.Submitted = true
			f.SubmitSuccess = true
		}

	case "reset":
		f.resetForm()
	}

	return nil
}

// calculatePasswordStrength returns a score 0-4
func (f *FormsWizard) calculatePasswordStrength(password string) int {
	if len(password) == 0 {
		return 0
	}

	score := 0

	// Length checks
	if len(password) >= 8 {
		score++
	}
	if len(password) >= 12 {
		score++
	}

	// Character type checks
	hasLower := false
	hasUpper := false
	hasDigit := false
	hasSpecial := false

	for _, c := range password {
		if unicode.IsLower(c) {
			hasLower = true
		} else if unicode.IsUpper(c) {
			hasUpper = true
		} else if unicode.IsDigit(c) {
			hasDigit = true
		} else {
			hasSpecial = true
		}
	}

	if hasLower && hasUpper {
		score++
	}
	if hasDigit || hasSpecial {
		score++
	}

	if score > 4 {
		score = 4
	}

	return score
}

// validateCurrentStep validates the current step
func (f *FormsWizard) validateCurrentStep() bool {
	f.Errors = nil

	switch f.CurrentStep {
	case StepBasics:
		return f.validateBasics()
	case StepProfile:
		return f.validateProfile()
	case StepPreferences:
		return f.validatePreferences()
	}

	return true
}

// validateBasics validates step 1
func (f *FormsWizard) validateBasics() bool {
	valid := true

	// Email validation
	if f.Email == "" {
		f.addError("email", "Email is required")
		valid = false
	} else if !isValidEmail(f.Email) {
		f.addError("email", "Please enter a valid email address")
		valid = false
	} else if !f.EmailAvailable {
		f.addError("email", "This email is already registered")
		valid = false
	}

	// Password validation
	if f.Password == "" {
		f.addError("password", "Password is required")
		valid = false
	} else if len(f.Password) < 8 {
		f.addError("password", "Password must be at least 8 characters")
		valid = false
	}

	// Password confirmation
	if f.PasswordConfirm != f.Password {
		f.addError("password_confirm", "Passwords do not match")
		valid = false
	}

	f.StepComplete[StepBasics] = valid
	return valid
}

// validateProfile validates step 2
func (f *FormsWizard) validateProfile() bool {
	valid := true

	// Full name
	if f.FullName == "" {
		f.addError("fullname", "Full name is required")
		valid = false
	} else if len(f.FullName) < 2 {
		f.addError("fullname", "Name must be at least 2 characters")
		valid = false
	}

	// Username
	if f.Username == "" {
		f.addError("username", "Username is required")
		valid = false
	} else if len(f.Username) < 3 {
		f.addError("username", "Username must be at least 3 characters")
		valid = false
	} else if !isValidUsername(f.Username) {
		f.addError("username", "Username can only contain letters, numbers, and underscores")
		valid = false
	} else if !f.UsernameAvailable {
		f.addError("username", "This username is already taken")
		valid = false
	}

	f.StepComplete[StepProfile] = valid
	return valid
}

// validatePreferences validates step 3
func (f *FormsWizard) validatePreferences() bool {
	f.StepComplete[StepPreferences] = true
	return true
}

// validateAllSteps validates all steps
func (f *FormsWizard) validateAllSteps() bool {
	origStep := f.CurrentStep

	f.CurrentStep = StepBasics
	if !f.validateBasics() {
		return false
	}

	f.CurrentStep = StepProfile
	if !f.validateProfile() {
		return false
	}

	f.CurrentStep = StepPreferences
	if !f.validatePreferences() {
		return false
	}

	f.CurrentStep = origStep
	return true
}

// addError adds a validation error
func (f *FormsWizard) addError(field, message string) {
	f.Errors = append(f.Errors, FieldError{Field: field, Message: message})
}

// removeError removes errors for a field
func (f *FormsWizard) removeError(field string) {
	var newErrors []FieldError
	for _, e := range f.Errors {
		if e.Field != field {
			newErrors = append(newErrors, e)
		}
	}
	f.Errors = newErrors
}

// getError returns the error message for a field, or empty string
func (f *FormsWizard) getError(field string) string {
	for _, e := range f.Errors {
		if e.Field == field {
			return e.Message
		}
	}
	return ""
}

// resetForm resets the form to initial state
func (f *FormsWizard) resetForm() {
	f.CurrentStep = StepBasics
	f.Email = ""
	f.Password = ""
	f.PasswordConfirm = ""
	f.PasswordStrength = 0
	f.FullName = ""
	f.Username = ""
	f.Bio = ""
	f.AvatarFilename = ""
	f.Theme = "system"
	f.Language = "en"
	f.Notifications = struct {
		Email   bool
		Push    bool
		SMS     bool
		Weekly  bool
	}{Email: true, Weekly: true}
	f.Errors = nil
	f.StepComplete = [4]bool{}
	f.Submitted = false
	f.SubmitSuccess = false
	f.CSRFToken = fmt.Sprintf("csrf_%d", time.Now().UnixNano())
}

// Helper functions
func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func isValidUsername(username string) bool {
	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	return usernameRegex.MatchString(username)
}

// Render returns the HTML representation.
func (f *FormsWizard) Render(ctx context.Context) core.Renderer {
	return core.RendererFunc(func(ctx context.Context, w io.Writer) error {
		html := f.renderWizard()
		_, err := w.Write([]byte(html))
		return err
	})
}

// renderWizard generates the complete wizard HTML
func (f *FormsWizard) renderWizard() string {
	cfg := website.PageConfig{
		Title:       "Multi-Step Form Wizard - GoliveKit Demo",
		Description: "Multi-step form with real-time validation, async checks, and CSRF protection.",
		URL:         "https://golivekit.cloud/demos/forms",
		Keywords:    []string{"forms", "validation", "wizard", "liveview"},
		Author:      "Gabriel Miguel",
		Language:    "en",
		ThemeColor:  "#8B5CF6",
	}

	body := f.renderWizardBody()
	return website.RenderDocument(cfg, renderFormsStyles(), body)
}

// renderFormsStyles returns custom CSS
func renderFormsStyles() string {
	return `
<style>
.wizard-container {
	max-width: 700px;
	margin: 0 auto;
	padding: 2rem;
}

.wizard-header {
	text-align: center;
	margin-bottom: 2rem;
}

.wizard-header h1 {
	font-size: 1.75rem;
	margin-bottom: 0.5rem;
}

.wizard-steps {
	display: flex;
	justify-content: center;
	align-items: center;
	margin-bottom: 2rem;
}

.step-item {
	display: flex;
	align-items: center;
}

.step-circle {
	width: 40px;
	height: 40px;
	border-radius: 50%;
	display: flex;
	align-items: center;
	justify-content: center;
	font-weight: 700;
	border: 2px solid var(--color-border);
	background: var(--color-bg);
	color: var(--color-textMuted);
	cursor: pointer;
	transition: all 0.3s;
}

.step-circle.active {
	border-color: var(--color-primary);
	background: var(--color-primary);
	color: white;
}

.step-circle.completed {
	border-color: var(--color-success);
	background: var(--color-success);
	color: white;
}

.step-label {
	font-size: 0.75rem;
	color: var(--color-textMuted);
	margin-top: 0.5rem;
	text-align: center;
}

.step-connector {
	width: 60px;
	height: 2px;
	background: var(--color-border);
	margin: 0 0.5rem;
}

.step-connector.active {
	background: var(--color-primary);
}

.wizard-card {
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 1rem;
	padding: 2rem;
}

.wizard-card-title {
	font-size: 1.25rem;
	margin-bottom: 0.25rem;
}

.wizard-card-subtitle {
	color: var(--color-textMuted);
	font-size: 0.875rem;
	margin-bottom: 1.5rem;
}

.form-group {
	margin-bottom: 1.5rem;
}

.form-label {
	display: block;
	font-weight: 600;
	margin-bottom: 0.5rem;
	font-size: 0.875rem;
}

.form-label .required {
	color: #ef4444;
}

.form-input-wrapper {
	position: relative;
}

.form-input {
	width: 100%;
	padding: 0.75rem 1rem;
	border: 1px solid var(--color-border);
	border-radius: 0.5rem;
	background: var(--color-bg);
	color: var(--color-text);
	font-size: 1rem;
	transition: border-color 0.2s;
}

.form-input:focus {
	outline: none;
	border-color: var(--color-primary);
}

.form-input.error {
	border-color: #ef4444;
}

.form-input.success {
	border-color: var(--color-success);
}

.input-icon {
	position: absolute;
	right: 1rem;
	top: 50%;
	transform: translateY(-50%);
	font-size: 1rem;
}

.input-icon.checking {
	animation: spin 1s linear infinite;
}

@keyframes spin {
	from { transform: translateY(-50%) rotate(0deg); }
	to { transform: translateY(-50%) rotate(360deg); }
}

.form-error {
	color: #ef4444;
	font-size: 0.75rem;
	margin-top: 0.375rem;
	display: flex;
	align-items: center;
	gap: 0.25rem;
}

.form-hint {
	color: var(--color-textMuted);
	font-size: 0.75rem;
	margin-top: 0.375rem;
}

.char-count {
	text-align: right;
	font-size: 0.75rem;
	color: var(--color-textMuted);
	margin-top: 0.25rem;
}

.password-strength {
	display: flex;
	gap: 0.25rem;
	margin-top: 0.5rem;
}

.strength-bar {
	flex: 1;
	height: 4px;
	background: var(--color-border);
	border-radius: 2px;
	transition: background 0.3s;
}

.strength-bar.active.weak { background: #ef4444; }
.strength-bar.active.fair { background: #f97316; }
.strength-bar.active.good { background: #eab308; }
.strength-bar.active.strong { background: var(--color-success); }

.strength-label {
	font-size: 0.75rem;
	margin-top: 0.25rem;
}

.checkbox-group {
	display: flex;
	flex-direction: column;
	gap: 0.75rem;
}

.checkbox-item {
	display: flex;
	align-items: center;
	gap: 0.75rem;
	cursor: pointer;
}

.checkbox-box {
	width: 20px;
	height: 20px;
	border: 2px solid var(--color-border);
	border-radius: 4px;
	display: flex;
	align-items: center;
	justify-content: center;
	transition: all 0.2s;
}

.checkbox-box.checked {
	background: var(--color-primary);
	border-color: var(--color-primary);
}

.checkbox-label {
	font-size: 0.9375rem;
}

.radio-group {
	display: flex;
	gap: 1rem;
	flex-wrap: wrap;
}

.radio-item {
	display: flex;
	align-items: center;
	gap: 0.5rem;
	padding: 0.75rem 1rem;
	border: 1px solid var(--color-border);
	border-radius: 0.5rem;
	cursor: pointer;
	transition: all 0.2s;
}

.radio-item:hover {
	border-color: var(--color-primary);
}

.radio-item.selected {
	border-color: var(--color-primary);
	background: rgba(139, 92, 246, 0.1);
}

.select-wrapper {
	position: relative;
}

.select-wrapper select {
	width: 100%;
	padding: 0.75rem 1rem;
	border: 1px solid var(--color-border);
	border-radius: 0.5rem;
	background: var(--color-bg);
	color: var(--color-text);
	font-size: 1rem;
	appearance: none;
	cursor: pointer;
}

.select-wrapper::after {
	content: "▼";
	position: absolute;
	right: 1rem;
	top: 50%;
	transform: translateY(-50%);
	font-size: 0.75rem;
	color: var(--color-textMuted);
	pointer-events: none;
}

.avatar-upload {
	display: flex;
	align-items: center;
	gap: 1.5rem;
}

.avatar-preview {
	width: 80px;
	height: 80px;
	border-radius: 50%;
	background: var(--color-bg);
	border: 2px solid var(--color-border);
	display: flex;
	align-items: center;
	justify-content: center;
	font-size: 2rem;
	color: var(--color-textMuted);
}

.avatar-info {
	flex: 1;
}

.upload-btn {
	padding: 0.5rem 1rem;
	border: 1px solid var(--color-border);
	border-radius: 0.5rem;
	background: var(--color-bg);
	cursor: pointer;
	font-size: 0.875rem;
	transition: all 0.2s;
}

.upload-btn:hover {
	border-color: var(--color-primary);
}

.upload-progress {
	margin-top: 0.5rem;
}

.progress-bar-container {
	height: 4px;
	background: var(--color-border);
	border-radius: 2px;
	overflow: hidden;
}

.progress-bar-fill {
	height: 100%;
	background: var(--color-primary);
	transition: width 0.3s;
}

.review-section {
	margin-bottom: 1.5rem;
	padding-bottom: 1.5rem;
	border-bottom: 1px solid var(--color-border);
}

.review-section:last-child {
	border-bottom: none;
}

.review-title {
	font-weight: 600;
	margin-bottom: 0.75rem;
	display: flex;
	align-items: center;
	gap: 0.5rem;
}

.review-item {
	display: flex;
	justify-content: space-between;
	padding: 0.375rem 0;
	font-size: 0.9375rem;
}

.review-label {
	color: var(--color-textMuted);
}

.review-value {
	font-weight: 500;
}

.wizard-actions {
	display: flex;
	justify-content: space-between;
	margin-top: 2rem;
	padding-top: 1.5rem;
	border-top: 1px solid var(--color-border);
}

.btn {
	padding: 0.75rem 1.5rem;
	border-radius: 0.5rem;
	font-weight: 600;
	cursor: pointer;
	transition: all 0.2s;
	border: none;
	font-size: 1rem;
}

.btn-primary {
	background: var(--color-primary);
	color: white;
}

.btn-primary:hover {
	background: #7c3aed;
}

.btn-secondary {
	background: var(--color-bg);
	color: var(--color-text);
	border: 1px solid var(--color-border);
}

.btn-secondary:hover {
	border-color: var(--color-primary);
}

.btn-success {
	background: var(--color-success);
	color: white;
}

.success-screen {
	text-align: center;
	padding: 3rem 2rem;
}

.success-icon {
	font-size: 4rem;
	margin-bottom: 1rem;
}

.success-title {
	font-size: 1.5rem;
	margin-bottom: 0.5rem;
}

.success-message {
	color: var(--color-textMuted);
	margin-bottom: 2rem;
}

.back-link {
	display: inline-flex;
	align-items: center;
	gap: 0.5rem;
	color: var(--color-textMuted);
	text-decoration: none;
	margin-bottom: 1rem;
	transition: color 0.2s;
}

.back-link:hover {
	color: var(--color-primary);
}

.csrf-notice {
	font-size: 0.75rem;
	color: var(--color-textMuted);
	text-align: center;
	margin-top: 1rem;
}
</style>
`
}

// renderWizardBody generates the main content
func (f *FormsWizard) renderWizardBody() string {
	// Navbar
	navbar := components.RenderNavbar(components.NavbarOptions{
		Logo:      "GoliveKit",
		LogoIcon:  "⚡",
		GitHubURL: "https://github.com/gabrielmiguelok/golivekit",
		ShowBadge: true,
		BadgeText: "Forms Demo",
		Links: []website.NavLink{
			{Label: "Demos", URL: "/demos", External: false},
			{Label: "Docs", URL: "/docs", External: false},
		},
	})

	// Show success screen if submitted
	if f.Submitted && f.SubmitSuccess {
		return navbar + f.renderSuccessScreen()
	}

	content := fmt.Sprintf(`
<main id="main-content">
<div class="wizard-container" data-live-view="forms-wizard">

<a href="/demos" class="back-link">← Back to Demos</a>

<div class="wizard-header">
	<h1>📝 Account Setup</h1>
	<p style="color:var(--color-textMuted)">Create your account in 4 simple steps</p>
</div>

%s

<div class="wizard-card">
	%s
</div>

</div>
</main>

<script src="/_live/golivekit.js"></script>
`, f.renderStepIndicator(), f.renderCurrentStep())

	return navbar + content
}

// renderStepIndicator renders the progress indicator
func (f *FormsWizard) renderStepIndicator() string {
	steps := []struct {
		label string
		icon  string
	}{
		{"Basics", "1"},
		{"Profile", "2"},
		{"Prefs", "3"},
		{"Review", "4"},
	}

	var html string
	for i, step := range steps {
		circleClass := ""
		if WizardStep(i) == f.CurrentStep {
			circleClass = "active"
		} else if f.StepComplete[i] || WizardStep(i) < f.CurrentStep {
			circleClass = "completed"
		}

		icon := step.icon
		if f.StepComplete[i] {
			icon = "✓"
		}

		html += fmt.Sprintf(`
<div class="step-item">
	<div style="display:flex;flex-direction:column;align-items:center">
		<div class="step-circle %s" lv-click="goto_step" lv-value-step="%d">%s</div>
		<div class="step-label">%s</div>
	</div>
</div>
`, circleClass, i, icon, step.label)

		if i < len(steps)-1 {
			connectorClass := ""
			if f.StepComplete[i] {
				connectorClass = "active"
			}
			html += fmt.Sprintf(`<div class="step-connector %s"></div>`, connectorClass)
		}
	}

	return `<div class="wizard-steps">` + html + `</div>`
}

// renderCurrentStep renders the current step content
func (f *FormsWizard) renderCurrentStep() string {
	switch f.CurrentStep {
	case StepBasics:
		return f.renderStepBasics()
	case StepProfile:
		return f.renderStepProfile()
	case StepPreferences:
		return f.renderStepPreferences()
	case StepReview:
		return f.renderStepReview()
	default:
		return ""
	}
}

// renderStepBasics renders Step 1
func (f *FormsWizard) renderStepBasics() string {
	// Email field status
	emailIcon := ""
	emailClass := ""
	if f.EmailChecking {
		emailIcon = `<span class="input-icon checking">⏳</span>`
	} else if f.Email != "" {
		if f.EmailAvailable && isValidEmail(f.Email) {
			emailIcon = `<span class="input-icon" style="color:var(--color-success)">✓</span>`
			emailClass = "success"
		} else if f.getError("email") != "" {
			emailIcon = `<span class="input-icon" style="color:#ef4444">✗</span>`
			emailClass = "error"
		}
	}

	emailError := ""
	if err := f.getError("email"); err != "" {
		emailError = fmt.Sprintf(`<div class="form-error">⚠️ %s</div>`, err)
	}

	// Password strength
	strengthLabels := []string{"", "Weak", "Fair", "Good", "Strong"}
	strengthColors := []string{"", "weak", "fair", "good", "strong"}
	strengthBars := ""
	for i := 1; i <= 4; i++ {
		active := ""
		if i <= f.PasswordStrength {
			active = fmt.Sprintf("active %s", strengthColors[f.PasswordStrength])
		}
		strengthBars += fmt.Sprintf(`<div class="strength-bar %s"></div>`, active)
	}

	strengthLabel := ""
	if f.PasswordStrength > 0 {
		color := ""
		switch f.PasswordStrength {
		case 1:
			color = "#ef4444"
		case 2:
			color = "#f97316"
		case 3:
			color = "#eab308"
		case 4:
			color = "var(--color-success)"
		}
		strengthLabel = fmt.Sprintf(`<div class="strength-label" style="color:%s">%s</div>`, color, strengthLabels[f.PasswordStrength])
	}

	passwordError := ""
	if err := f.getError("password"); err != "" {
		passwordError = fmt.Sprintf(`<div class="form-error">⚠️ %s</div>`, err)
	}

	confirmError := ""
	if err := f.getError("password_confirm"); err != "" {
		confirmError = fmt.Sprintf(`<div class="form-error">⚠️ %s</div>`, err)
	}

	return fmt.Sprintf(`
<h2 class="wizard-card-title">Step 1: Basic Information</h2>
<p class="wizard-card-subtitle">Let's start with your email and password</p>

<div class="form-group">
	<label class="form-label">Email Address <span class="required">*</span></label>
	<div class="form-input-wrapper">
		<input type="email" class="form-input %s" placeholder="you@example.com"
			lv-change="update_email" lv-debounce="300"
			lv-blur="check_email"
			value="%s">
		%s
	</div>
	%s
</div>

<div class="form-group">
	<label class="form-label">Password <span class="required">*</span></label>
	<div class="form-input-wrapper">
		<input type="password" class="form-input" placeholder="Enter password"
			lv-change="update_password" lv-debounce="150"
			value="%s">
	</div>
	<div class="password-strength">%s</div>
	%s
	%s
	<div class="form-hint">Use 8+ characters with mixed case, numbers, and symbols</div>
</div>

<div class="form-group">
	<label class="form-label">Confirm Password <span class="required">*</span></label>
	<div class="form-input-wrapper">
		<input type="password" class="form-input" placeholder="Confirm password"
			lv-change="update_password_confirm" lv-debounce="150"
			value="%s">
	</div>
	%s
</div>

<div class="wizard-actions">
	<div></div>
	<button class="btn btn-primary" lv-click="next_step">Continue →</button>
</div>
`, emailClass, f.Email, emailIcon, emailError, f.Password, strengthBars, strengthLabel, passwordError, f.PasswordConfirm, confirmError)
}

// renderStepProfile renders Step 2
func (f *FormsWizard) renderStepProfile() string {
	// Fullname error
	fullnameError := ""
	if err := f.getError("fullname"); err != "" {
		fullnameError = fmt.Sprintf(`<div class="form-error">⚠️ %s</div>`, err)
	}

	// Username field status
	usernameIcon := ""
	usernameClass := ""
	if f.UsernameChecking {
		usernameIcon = `<span class="input-icon checking">⏳</span>`
	} else if f.Username != "" {
		if f.UsernameAvailable && isValidUsername(f.Username) && len(f.Username) >= 3 {
			usernameIcon = `<span class="input-icon" style="color:var(--color-success)">✓</span>`
			usernameClass = "success"
		} else if f.getError("username") != "" {
			usernameIcon = `<span class="input-icon" style="color:#ef4444">✗</span>`
			usernameClass = "error"
		}
	}

	usernameError := ""
	if err := f.getError("username"); err != "" {
		usernameError = fmt.Sprintf(`<div class="form-error">⚠️ %s</div>`, err)
	}

	// Avatar preview
	avatarContent := "👤"
	avatarInfo := `<button class="upload-btn" lv-click="upload_avatar" lv-value-filename="avatar.jpg">Choose File</button>`
	if f.AvatarFilename != "" {
		avatarContent = "🖼️"
		if f.AvatarProgress < 100 {
			avatarInfo = fmt.Sprintf(`
<span>%s</span>
<div class="upload-progress">
	<div class="progress-bar-container">
		<div class="progress-bar-fill" style="width:%d%%"></div>
	</div>
	<span style="font-size:0.75rem;color:var(--color-textMuted)">%d%% uploaded</span>
</div>
`, f.AvatarFilename, f.AvatarProgress, f.AvatarProgress)
		} else {
			avatarInfo = fmt.Sprintf(`
<span style="color:var(--color-success)">✓ %s</span>
<button class="upload-btn" lv-click="upload_avatar" lv-value-filename="">Change</button>
`, f.AvatarFilename)
		}
	}

	return fmt.Sprintf(`
<h2 class="wizard-card-title">Step 2: Your Profile</h2>
<p class="wizard-card-subtitle">Tell us a bit about yourself</p>

<div class="form-group">
	<label class="form-label">Full Name <span class="required">*</span></label>
	<input type="text" class="form-input" placeholder="John Doe"
		lv-change="update_fullname" lv-debounce="150"
		value="%s">
	%s
</div>

<div class="form-group">
	<label class="form-label">Username <span class="required">*</span></label>
	<div class="form-input-wrapper">
		<input type="text" class="form-input %s" placeholder="johndoe"
			lv-change="update_username" lv-debounce="300"
			lv-blur="check_username"
			value="%s">
		%s
	</div>
	%s
	<div class="form-hint">Letters, numbers, and underscores only</div>
</div>

<div class="form-group">
	<label class="form-label">Bio (optional)</label>
	<textarea class="form-input" placeholder="Tell us about yourself..." rows="3"
		lv-change="update_bio" lv-debounce="150"
		maxlength="500">%s</textarea>
	<div class="char-count">%d/500 characters</div>
</div>

<div class="form-group">
	<label class="form-label">Avatar (optional)</label>
	<div class="avatar-upload">
		<div class="avatar-preview">%s</div>
		<div class="avatar-info">%s</div>
	</div>
</div>

<div class="wizard-actions">
	<button class="btn btn-secondary" lv-click="prev_step">← Back</button>
	<button class="btn btn-primary" lv-click="next_step">Continue →</button>
</div>
`, f.FullName, fullnameError, usernameClass, f.Username, usernameIcon, usernameError, f.Bio, len(f.Bio), avatarContent, avatarInfo)
}

// renderStepPreferences renders Step 3
func (f *FormsWizard) renderStepPreferences() string {
	// Theme selection
	themeOptions := []struct {
		value string
		label string
		icon  string
	}{
		{"light", "Light", "☀️"},
		{"dark", "Dark", "🌙"},
		{"system", "System", "💻"},
	}

	themeHTML := ""
	for _, opt := range themeOptions {
		selected := ""
		if f.Theme == opt.value {
			selected = "selected"
		}
		themeHTML += fmt.Sprintf(`
<div class="radio-item %s" lv-click="update_theme" lv-value-value="%s">
	<span>%s</span>
	<span>%s</span>
</div>
`, selected, opt.value, opt.icon, opt.label)
	}

	// Notification checkboxes
	notifHTML := ""
	notifs := []struct {
		key     string
		label   string
		checked bool
	}{
		{"email", "Email notifications", f.Notifications.Email},
		{"push", "Push notifications", f.Notifications.Push},
		{"sms", "SMS notifications", f.Notifications.SMS},
		{"weekly", "Weekly digest", f.Notifications.Weekly},
	}

	for _, n := range notifs {
		checked := ""
		if n.checked {
			checked = "checked"
		}
		notifHTML += fmt.Sprintf(`
<div class="checkbox-item" lv-click="toggle_notification" lv-value-type="%s">
	<div class="checkbox-box %s">%s</div>
	<span class="checkbox-label">%s</span>
</div>
`, n.key, checked, func() string {
			if n.checked {
				return "✓"
			}
			return ""
		}(), n.label)
	}

	return fmt.Sprintf(`
<h2 class="wizard-card-title">Step 3: Preferences</h2>
<p class="wizard-card-subtitle">Customize your experience</p>

<div class="form-group">
	<label class="form-label">Theme</label>
	<div class="radio-group">
		%s
	</div>
</div>

<div class="form-group">
	<label class="form-label">Language</label>
	<div class="select-wrapper">
		<select lv-change="update_language">
			<option value="en" %s>English</option>
			<option value="es" %s>Español</option>
			<option value="fr" %s>Français</option>
			<option value="de" %s>Deutsch</option>
			<option value="ja" %s>日本語</option>
		</select>
	</div>
</div>

<div class="form-group">
	<label class="form-label">Notifications</label>
	<div class="checkbox-group">
		%s
	</div>
</div>

<div class="wizard-actions">
	<button class="btn btn-secondary" lv-click="prev_step">← Back</button>
	<button class="btn btn-primary" lv-click="next_step">Continue →</button>
</div>
`, themeHTML,
		selected("en", f.Language), selected("es", f.Language), selected("fr", f.Language),
		selected("de", f.Language), selected("ja", f.Language), notifHTML)
}

func selected(val, current string) string {
	if val == current {
		return "selected"
	}
	return ""
}

// renderStepReview renders Step 4
func (f *FormsWizard) renderStepReview() string {
	// Format notifications
	notifList := []string{}
	if f.Notifications.Email {
		notifList = append(notifList, "Email")
	}
	if f.Notifications.Push {
		notifList = append(notifList, "Push")
	}
	if f.Notifications.SMS {
		notifList = append(notifList, "SMS")
	}
	if f.Notifications.Weekly {
		notifList = append(notifList, "Weekly")
	}
	notifStr := "None"
	if len(notifList) > 0 {
		notifStr = strings.Join(notifList, ", ")
	}

	// Language names
	langNames := map[string]string{
		"en": "English", "es": "Español", "fr": "Français",
		"de": "Deutsch", "ja": "日本語",
	}

	// Theme names
	themeNames := map[string]string{
		"light": "Light", "dark": "Dark", "system": "System",
	}

	return fmt.Sprintf(`
<h2 class="wizard-card-title">Step 4: Review & Submit</h2>
<p class="wizard-card-subtitle">Please review your information before submitting</p>

<div class="review-section">
	<div class="review-title">📧 Basic Information</div>
	<div class="review-item">
		<span class="review-label">Email</span>
		<span class="review-value">%s</span>
	</div>
	<div class="review-item">
		<span class="review-label">Password</span>
		<span class="review-value">••••••••</span>
	</div>
</div>

<div class="review-section">
	<div class="review-title">👤 Profile</div>
	<div class="review-item">
		<span class="review-label">Full Name</span>
		<span class="review-value">%s</span>
	</div>
	<div class="review-item">
		<span class="review-label">Username</span>
		<span class="review-value">@%s</span>
	</div>
	<div class="review-item">
		<span class="review-label">Bio</span>
		<span class="review-value">%s</span>
	</div>
	<div class="review-item">
		<span class="review-label">Avatar</span>
		<span class="review-value">%s</span>
	</div>
</div>

<div class="review-section">
	<div class="review-title">⚙️ Preferences</div>
	<div class="review-item">
		<span class="review-label">Theme</span>
		<span class="review-value">%s</span>
	</div>
	<div class="review-item">
		<span class="review-label">Language</span>
		<span class="review-value">%s</span>
	</div>
	<div class="review-item">
		<span class="review-label">Notifications</span>
		<span class="review-value">%s</span>
	</div>
</div>

<input type="hidden" name="csrf_token" value="%s">
<div class="csrf-notice">🔐 Protected with CSRF token</div>

<div class="wizard-actions">
	<button class="btn btn-secondary" lv-click="prev_step">← Back</button>
	<button class="btn btn-success" lv-click="submit">✓ Create Account</button>
</div>
`, f.Email, f.FullName, f.Username,
		func() string {
			if f.Bio != "" {
				if len(f.Bio) > 50 {
					return f.Bio[:50] + "..."
				}
				return f.Bio
			}
			return "Not set"
		}(),
		func() string {
			if f.AvatarFilename != "" {
				return f.AvatarFilename
			}
			return "Not uploaded"
		}(),
		themeNames[f.Theme], langNames[f.Language], notifStr, f.CSRFToken)
}

// renderSuccessScreen renders the success message
func (f *FormsWizard) renderSuccessScreen() string {
	return fmt.Sprintf(`
<main id="main-content">
<div class="wizard-container" data-live-view="forms-wizard">

<a href="/demos" class="back-link">← Back to Demos</a>

<div class="wizard-card">
	<div class="success-screen">
		<div class="success-icon">🎉</div>
		<h2 class="success-title">Account Created Successfully!</h2>
		<p class="success-message">
			Welcome, <strong>%s</strong>! Your account has been created.<br>
			Check your email at <strong>%s</strong> for verification.
		</p>
		<button class="btn btn-primary" lv-click="reset">Create Another Account</button>
	</div>
</div>

</div>
</main>

<script src="/_live/golivekit.js"></script>
`, f.FullName, f.Email)
}
