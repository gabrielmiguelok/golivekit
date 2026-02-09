// Package js provides JavaScript command utilities for GoliveKit.
// These commands execute on the client without a server roundtrip.
package js

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Command represents a JavaScript command to execute on the client.
type Command interface {
	// ToJS returns the JavaScript code to execute.
	ToJS() string
}

// Commands holds a sequence of commands.
type Commands []Command

// ToJS returns the JavaScript for all commands.
func (cs Commands) ToJS() string {
	var parts []string
	for _, c := range cs {
		parts = append(parts, c.ToJS())
	}
	return strings.Join(parts, ";")
}

// String implements fmt.Stringer.
func (cs Commands) String() string {
	return cs.ToJS()
}

// jsCommand is a simple command holder.
type jsCommand struct {
	code string
}

func (c jsCommand) ToJS() string {
	return c.code
}

func (c jsCommand) String() string {
	return c.code
}

// JS is the namespace for JavaScript commands.
var JS = jsNamespace{}

type jsNamespace struct{}

// Show shows an element.
func (js jsNamespace) Show(selector string, opts ...ShowOption) Command {
	config := showConfig{
		transition: "",
		time:       200,
	}
	for _, opt := range opts {
		opt(&config)
	}

	if config.transition != "" {
		return jsCommand{code: fmt.Sprintf(`liveview.JS.show("%s",{transition:"%s",time:%d})`, selector, config.transition, config.time)}
	}
	return jsCommand{code: fmt.Sprintf(`liveview.JS.show("%s")`, selector)}
}

// Hide hides an element.
func (js jsNamespace) Hide(selector string, opts ...HideOption) Command {
	config := hideConfig{
		transition: "",
		time:       200,
	}
	for _, opt := range opts {
		opt(&config)
	}

	if config.transition != "" {
		return jsCommand{code: fmt.Sprintf(`liveview.JS.hide("%s",{transition:"%s",time:%d})`, selector, config.transition, config.time)}
	}
	return jsCommand{code: fmt.Sprintf(`liveview.JS.hide("%s")`, selector)}
}

// Toggle toggles element visibility.
func (js jsNamespace) Toggle(selector string, opts ...ToggleOption) Command {
	config := toggleConfig{
		inTransition:  "",
		outTransition: "",
		time:          200,
	}
	for _, opt := range opts {
		opt(&config)
	}

	if config.inTransition != "" || config.outTransition != "" {
		return jsCommand{code: fmt.Sprintf(`liveview.JS.toggle("%s",{in:"%s",out:"%s",time:%d})`,
			selector, config.inTransition, config.outTransition, config.time)}
	}
	return jsCommand{code: fmt.Sprintf(`liveview.JS.toggle("%s")`, selector)}
}

// AddClass adds CSS class(es) to an element.
func (js jsNamespace) AddClass(selector, class string, opts ...ClassOption) Command {
	config := classConfig{
		transition: "",
		time:       200,
	}
	for _, opt := range opts {
		opt(&config)
	}

	if config.transition != "" {
		return jsCommand{code: fmt.Sprintf(`liveview.JS.addClass("%s","%s",{transition:"%s",time:%d})`,
			selector, class, config.transition, config.time)}
	}
	return jsCommand{code: fmt.Sprintf(`liveview.JS.addClass("%s","%s")`, selector, class)}
}

// RemoveClass removes CSS class(es) from an element.
func (js jsNamespace) RemoveClass(selector, class string, opts ...ClassOption) Command {
	config := classConfig{
		transition: "",
		time:       200,
	}
	for _, opt := range opts {
		opt(&config)
	}

	if config.transition != "" {
		return jsCommand{code: fmt.Sprintf(`liveview.JS.removeClass("%s","%s",{transition:"%s",time:%d})`,
			selector, class, config.transition, config.time)}
	}
	return jsCommand{code: fmt.Sprintf(`liveview.JS.removeClass("%s","%s")`, selector, class)}
}

// ToggleClass toggles CSS class(es) on an element.
func (js jsNamespace) ToggleClass(selector, class string) Command {
	return jsCommand{code: fmt.Sprintf(`liveview.JS.toggleClass("%s","%s")`, selector, class)}
}

// SetAttr sets an attribute on an element.
func (js jsNamespace) SetAttr(selector, attr, value string) Command {
	return jsCommand{code: fmt.Sprintf(`liveview.JS.setAttr("%s","%s","%s")`, selector, attr, value)}
}

// RemoveAttr removes an attribute from an element.
func (js jsNamespace) RemoveAttr(selector, attr string) Command {
	return jsCommand{code: fmt.Sprintf(`liveview.JS.removeAttr("%s","%s")`, selector, attr)}
}

// ToggleAttr toggles an attribute on an element.
func (js jsNamespace) ToggleAttr(selector, attr string, values ...string) Command {
	if len(values) == 2 {
		return jsCommand{code: fmt.Sprintf(`liveview.JS.toggleAttr("%s","%s",["%s","%s"])`,
			selector, attr, values[0], values[1])}
	}
	return jsCommand{code: fmt.Sprintf(`liveview.JS.toggleAttr("%s","%s")`, selector, attr)}
}

// Dispatch dispatches a DOM event on an element.
func (js jsNamespace) Dispatch(selector, event string, opts ...DispatchOption) Command {
	config := dispatchConfig{}
	for _, opt := range opts {
		opt(&config)
	}

	if config.detail != nil {
		detailJSON, _ := json.Marshal(config.detail)
		return jsCommand{code: fmt.Sprintf(`liveview.JS.dispatch("%s","%s",{detail:%s})`,
			selector, event, string(detailJSON))}
	}
	return jsCommand{code: fmt.Sprintf(`liveview.JS.dispatch("%s","%s")`, selector, event)}
}

// Focus sets focus on an element.
func (js jsNamespace) Focus(selector string) Command {
	return jsCommand{code: fmt.Sprintf(`liveview.JS.focus("%s")`, selector)}
}

// FocusFirst focuses the first focusable element in a container.
func (js jsNamespace) FocusFirst(selector string) Command {
	return jsCommand{code: fmt.Sprintf(`liveview.JS.focusFirst("%s")`, selector)}
}

// PushFocus pushes focus state before changing it.
func (js jsNamespace) PushFocus(selector string) Command {
	return jsCommand{code: fmt.Sprintf(`liveview.JS.pushFocus("%s")`, selector)}
}

// PopFocus restores previous focus state.
func (js jsNamespace) PopFocus() Command {
	return jsCommand{code: `liveview.JS.popFocus()`}
}

// Push sends an event to the server.
func (js jsNamespace) Push(event string, opts ...PushOption) Command {
	config := pushConfig{
		value: make(map[string]any),
	}
	for _, opt := range opts {
		opt(&config)
	}

	valueJSON, _ := json.Marshal(config.value)
	if config.target != "" {
		return jsCommand{code: fmt.Sprintf(`liveview.JS.push("%s",{value:%s,target:"%s"})`,
			event, string(valueJSON), config.target)}
	}
	return jsCommand{code: fmt.Sprintf(`liveview.JS.push("%s",{value:%s})`, event, string(valueJSON))}
}

// Navigate navigates to a new page.
func (js jsNamespace) Navigate(path string, opts ...NavigateOption) Command {
	config := navigateConfig{}
	for _, opt := range opts {
		opt(&config)
	}

	if config.replace {
		return jsCommand{code: fmt.Sprintf(`liveview.JS.navigate("%s",{replace:true})`, path)}
	}
	return jsCommand{code: fmt.Sprintf(`liveview.JS.navigate("%s")`, path)}
}

// Patch patches the current URL.
func (js jsNamespace) Patch(path string, opts ...PatchOption) Command {
	config := patchConfig{}
	for _, opt := range opts {
		opt(&config)
	}

	if config.replace {
		return jsCommand{code: fmt.Sprintf(`liveview.JS.patch("%s",{replace:true})`, path)}
	}
	return jsCommand{code: fmt.Sprintf(`liveview.JS.patch("%s")`, path)}
}

// Exec executes arbitrary JavaScript.
func (js jsNamespace) Exec(code string) Command {
	return jsCommand{code: code}
}

// Pipe chains multiple commands.
func (js jsNamespace) Pipe(commands ...Command) Command {
	var parts []string
	for _, cmd := range commands {
		parts = append(parts, cmd.ToJS())
	}
	return jsCommand{code: strings.Join(parts, ";")}
}

// Option types

type showConfig struct {
	transition string
	time       int
}

type ShowOption func(*showConfig)

func Transition(name string) ShowOption {
	return func(c *showConfig) {
		c.transition = name
	}
}

func Time(ms int) ShowOption {
	return func(c *showConfig) {
		c.time = ms
	}
}

type hideConfig struct {
	transition string
	time       int
}

type HideOption func(*hideConfig)

type toggleConfig struct {
	inTransition  string
	outTransition string
	time          int
}

type ToggleOption func(*toggleConfig)

func InTransition(name string) ToggleOption {
	return func(c *toggleConfig) {
		c.inTransition = name
	}
}

func OutTransition(name string) ToggleOption {
	return func(c *toggleConfig) {
		c.outTransition = name
	}
}

type classConfig struct {
	transition string
	time       int
}

type ClassOption func(*classConfig)

type dispatchConfig struct {
	detail map[string]any
}

type DispatchOption func(*dispatchConfig)

func Detail(d map[string]any) DispatchOption {
	return func(c *dispatchConfig) {
		c.detail = d
	}
}

type pushConfig struct {
	value  map[string]any
	target string
}

type PushOption func(*pushConfig)

func Value(v map[string]any) PushOption {
	return func(c *pushConfig) {
		c.value = v
	}
}

func Target(t string) PushOption {
	return func(c *pushConfig) {
		c.target = t
	}
}

type navigateConfig struct {
	replace bool
}

type NavigateOption func(*navigateConfig)

func Replace() NavigateOption {
	return func(c *navigateConfig) {
		c.replace = true
	}
}

type patchConfig struct {
	replace bool
}

type PatchOption func(*patchConfig)

// Common transitions
const (
	TransitionFadeIn    = "fade-in"
	TransitionFadeOut   = "fade-out"
	TransitionSlideIn   = "slide-in"
	TransitionSlideOut  = "slide-out"
	TransitionScaleIn   = "scale-in"
	TransitionScaleOut  = "scale-out"
)
