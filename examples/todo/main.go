// Package main demonstrates a todo list using GoliveKit.
package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gabrielmiguelok/golivekit/pkg/core"
	"github.com/gabrielmiguelok/golivekit/pkg/forms"
)

func main() {
	// Create todo list component
	todoList := NewTodoList()

	// HTTP handlers
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/api/todos", todoList.HandleAPI)

	fmt.Println("Todo server starting at http://localhost:3001")
	log.Fatal(http.ListenAndServe(":3001", nil))
}

// Todo represents a todo item.
type Todo struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Completed   bool      `json:"completed"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TodoList is a LiveView component for managing todos.
type TodoList struct {
	core.BaseComponent

	todos      []Todo
	filter     string // all, active, completed
	editingID  string
	form       *forms.Form
	changeset  *forms.Changeset
	mu         sync.RWMutex
}

// NewTodoList creates a new todo list component.
func NewTodoList() *TodoList {
	// Create form for adding todos
	form := forms.NewForm("todo_form")
	form.AddField(forms.Field{
		Name:        "title",
		Type:        forms.FieldText,
		Label:       "Title",
		Required:    true,
		Placeholder: "What needs to be done?",
		Validators:  []forms.Validator{forms.Required(), forms.MinLength(1), forms.MaxLength(200)},
	})
	form.AddField(forms.Field{
		Name:        "description",
		Type:        forms.FieldTextarea,
		Label:       "Description",
		Placeholder: "Optional description...",
		Validators:  []forms.Validator{forms.MaxLength(1000)},
	})

	return &TodoList{
		todos:  make([]Todo, 0),
		filter: "all",
		form:   form,
	}
}

// Name returns the component name.
func (c *TodoList) Name() string {
	return "TodoList"
}

// Mount initializes the component.
func (c *TodoList) Mount(ctx context.Context, params core.Params, session core.Session) error {
	// Load initial todos (in a real app, this would come from a database)
	c.todos = []Todo{
		{
			ID:        "1",
			Title:     "Learn GoliveKit",
			Completed: false,
			CreatedAt: time.Now().Add(-24 * time.Hour),
			UpdatedAt: time.Now().Add(-24 * time.Hour),
		},
		{
			ID:        "2",
			Title:     "Build a real-time app",
			Completed: false,
			CreatedAt: time.Now().Add(-12 * time.Hour),
			UpdatedAt: time.Now().Add(-12 * time.Hour),
		},
		{
			ID:          "3",
			Title:       "Deploy to production",
			Description: "Use Docker for deployment",
			Completed:   true,
			CreatedAt:   time.Now().Add(-6 * time.Hour),
			UpdatedAt:   time.Now(),
		},
	}

	// Initialize changeset
	c.changeset = forms.Cast(nil, nil, []string{"title", "description"})

	return nil
}

// Render returns the component HTML.
func (c *TodoList) Render(ctx context.Context) core.Renderer {
	return core.RendererFunc(func(ctx context.Context, w io.Writer) error {
		c.mu.RLock()
		defer c.mu.RUnlock()

		// Filter todos
		filtered := c.filteredTodos()

		// Count stats
		total := len(c.todos)
		completed := 0
		for _, t := range c.todos {
			if t.Completed {
				completed++
			}
		}

		data := map[string]any{
			"Todos":     filtered,
			"Filter":    c.filter,
			"Total":     total,
			"Active":    total - completed,
			"Completed": completed,
			"EditingID": c.editingID,
			"Form":      c.form,
			"Changeset": c.changeset,
		}

		return todoTemplate.Execute(w, data)
	})
}

// HandleEvent handles user events.
func (c *TodoList) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	switch event {
	case "add":
		return c.handleAdd(ctx, payload)
	case "toggle":
		return c.handleToggle(ctx, payload)
	case "delete":
		return c.handleDelete(ctx, payload)
	case "edit":
		return c.handleEdit(ctx, payload)
	case "save":
		return c.handleSave(ctx, payload)
	case "cancel":
		return c.handleCancel(ctx)
	case "filter":
		return c.handleFilter(ctx, payload)
	case "clear_completed":
		return c.handleClearCompleted(ctx)
	case "validate":
		return c.handleValidate(ctx, payload)
	}
	return nil
}

func (c *TodoList) handleAdd(ctx context.Context, payload map[string]any) error {
	title, _ := payload["title"].(string)
	description, _ := payload["description"].(string)

	// Validate using changeset
	changeset := forms.Cast(nil, map[string]any{
		"title":       title,
		"description": description,
	}, []string{"title", "description"})

	changeset = changeset.
		ValidateRequired("title").
		ValidateLength("title", forms.LengthOpts{Min: 1, Max: 200})

	if !changeset.Valid {
		c.changeset = changeset
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	todo := Todo{
		ID:          fmt.Sprintf("%d", time.Now().UnixNano()),
		Title:       title,
		Description: description,
		Completed:   false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	c.todos = append(c.todos, todo)
	c.changeset = forms.Cast(nil, nil, []string{"title", "description"})
	c.form.Reset()

	return nil
}

func (c *TodoList) handleToggle(ctx context.Context, payload map[string]any) error {
	id, _ := payload["id"].(string)

	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.todos {
		if c.todos[i].ID == id {
			c.todos[i].Completed = !c.todos[i].Completed
			c.todos[i].UpdatedAt = time.Now()
			break
		}
	}

	return nil
}

func (c *TodoList) handleDelete(ctx context.Context, payload map[string]any) error {
	id, _ := payload["id"].(string)

	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.todos {
		if c.todos[i].ID == id {
			c.todos = append(c.todos[:i], c.todos[i+1:]...)
			break
		}
	}

	return nil
}

func (c *TodoList) handleEdit(ctx context.Context, payload map[string]any) error {
	id, _ := payload["id"].(string)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.editingID = id

	// Find todo and populate form
	for _, todo := range c.todos {
		if todo.ID == id {
			c.form.SetValue("title", todo.Title)
			c.form.SetValue("description", todo.Description)
			break
		}
	}

	return nil
}

func (c *TodoList) handleSave(ctx context.Context, payload map[string]any) error {
	title, _ := payload["title"].(string)
	description, _ := payload["description"].(string)

	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.todos {
		if c.todos[i].ID == c.editingID {
			c.todos[i].Title = title
			c.todos[i].Description = description
			c.todos[i].UpdatedAt = time.Now()
			break
		}
	}

	c.editingID = ""
	c.form.Reset()

	return nil
}

func (c *TodoList) handleCancel(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.editingID = ""
	c.form.Reset()

	return nil
}

func (c *TodoList) handleFilter(ctx context.Context, payload map[string]any) error {
	filter, _ := payload["filter"].(string)

	c.mu.Lock()
	defer c.mu.Unlock()

	if filter == "all" || filter == "active" || filter == "completed" {
		c.filter = filter
	}

	return nil
}

func (c *TodoList) handleClearCompleted(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	filtered := make([]Todo, 0)
	for _, todo := range c.todos {
		if !todo.Completed {
			filtered = append(filtered, todo)
		}
	}
	c.todos = filtered

	return nil
}

func (c *TodoList) handleValidate(ctx context.Context, payload map[string]any) error {
	// Real-time validation
	changeset := forms.Cast(nil, payload, []string{"title", "description"})
	changeset = changeset.
		ValidateRequired("title").
		ValidateLength("title", forms.LengthOpts{Min: 1, Max: 200})

	c.mu.Lock()
	c.changeset = changeset
	c.mu.Unlock()

	return nil
}

func (c *TodoList) filteredTodos() []Todo {
	filtered := make([]Todo, 0)
	for _, todo := range c.todos {
		switch c.filter {
		case "active":
			if !todo.Completed {
				filtered = append(filtered, todo)
			}
		case "completed":
			if todo.Completed {
				filtered = append(filtered, todo)
			}
		default:
			filtered = append(filtered, todo)
		}
	}
	return filtered
}

// HandleAPI handles REST API requests.
func (c *TodoList) HandleAPI(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	count := len(c.todos)
	c.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"count": %d}`, count)
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	homeTemplate.Execute(w, nil)
}

var homeTemplate = template.Must(template.New("home").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>GoliveKit Todo Example</title>
    <style>
        * {
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            max-width: 600px;
            margin: 0 auto;
            padding: 2rem;
            background: #f5f5f5;
        }
        h1 {
            text-align: center;
            color: #333;
        }
        .todo-app {
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        .input-area {
            padding: 1rem;
            border-bottom: 1px solid #eee;
        }
        .input-area input {
            width: 100%;
            padding: 0.75rem;
            font-size: 1rem;
            border: 1px solid #ddd;
            border-radius: 4px;
            margin-bottom: 0.5rem;
        }
        .input-area textarea {
            width: 100%;
            padding: 0.75rem;
            font-size: 0.9rem;
            border: 1px solid #ddd;
            border-radius: 4px;
            resize: vertical;
            min-height: 60px;
        }
        .input-area button {
            margin-top: 0.5rem;
            padding: 0.5rem 1rem;
            background: #007bff;
            color: white;
            border: none;
            border-radius: 4px;
            cursor: pointer;
        }
        .todo-list {
            list-style: none;
            padding: 0;
            margin: 0;
        }
        .todo-item {
            display: flex;
            align-items: center;
            padding: 0.75rem 1rem;
            border-bottom: 1px solid #eee;
        }
        .todo-item:last-child {
            border-bottom: none;
        }
        .todo-item input[type="checkbox"] {
            margin-right: 1rem;
            transform: scale(1.2);
        }
        .todo-item.completed .todo-text {
            text-decoration: line-through;
            color: #999;
        }
        .todo-text {
            flex: 1;
        }
        .todo-actions {
            display: flex;
            gap: 0.5rem;
        }
        .todo-actions button {
            padding: 0.25rem 0.5rem;
            font-size: 0.8rem;
            border: none;
            border-radius: 4px;
            cursor: pointer;
        }
        .todo-actions .edit {
            background: #6c757d;
            color: white;
        }
        .todo-actions .delete {
            background: #dc3545;
            color: white;
        }
        .filters {
            display: flex;
            justify-content: space-between;
            padding: 1rem;
            background: #f9f9f9;
            font-size: 0.9rem;
        }
        .filters button {
            background: none;
            border: none;
            cursor: pointer;
            color: #666;
        }
        .filters button.active {
            color: #007bff;
            font-weight: bold;
        }
        .error {
            color: #dc3545;
            font-size: 0.8rem;
            margin-top: 0.25rem;
        }
    </style>
</head>
<body>
    <h1>GoliveKit Todo</h1>

    <div class="todo-app">
        <div class="input-area">
            <input type="text" id="title" placeholder="What needs to be done?" />
            <textarea id="description" placeholder="Optional description..."></textarea>
            <button onclick="addTodo()">Add Todo</button>
        </div>

        <ul class="todo-list" id="todos">
            <li class="todo-item">
                <input type="checkbox" />
                <span class="todo-text">Example todo item</span>
                <div class="todo-actions">
                    <button class="edit">Edit</button>
                    <button class="delete">Delete</button>
                </div>
            </li>
        </ul>

        <div class="filters">
            <span id="count">0 items left</span>
            <div>
                <button class="active" onclick="filter('all')">All</button>
                <button onclick="filter('active')">Active</button>
                <button onclick="filter('completed')">Completed</button>
            </div>
            <button onclick="clearCompleted()">Clear completed</button>
        </div>
    </div>

    <script>
        // In a real implementation, this would connect to the LiveView socket
        console.log('Todo app ready - connect via WebSocket for real-time updates');

        function addTodo() {
            const title = document.getElementById('title').value.trim();
            const description = document.getElementById('description').value.trim();
            if (title) {
                console.log('Add:', { title, description });
                document.getElementById('title').value = '';
                document.getElementById('description').value = '';
            }
        }

        function filter(type) {
            console.log('Filter:', type);
        }

        function clearCompleted() {
            console.log('Clear completed');
        }

        document.getElementById('title').addEventListener('keypress', function(e) {
            if (e.key === 'Enter') addTodo();
        });
    </script>
</body>
</html>`))

var todoTemplate = template.Must(template.New("todo").Parse(`
<div class="input-area">
    <input type="text"
           name="title"
           value="{{if .Changeset}}{{index .Changeset.Changes "title"}}{{end}}"
           placeholder="What needs to be done?"
           lv-change="validate"
           lv-debounce="300" />
    {{if .Changeset}}{{range index .Changeset.Errors "title"}}
    <div class="error">{{.}}</div>
    {{end}}{{end}}
    <textarea name="description"
              placeholder="Optional description..."
              lv-change="validate"
              lv-debounce="300">{{if .Changeset}}{{index .Changeset.Changes "description"}}{{end}}</textarea>
    <button lv-click="add">Add Todo</button>
</div>

<ul class="todo-list">
    {{range .Todos}}
    <li class="todo-item {{if .Completed}}completed{{end}}">
        {{if eq $.EditingID .ID}}
        <input type="text" name="edit_title" value="{{.Title}}" />
        <button lv-click="save">Save</button>
        <button lv-click="cancel">Cancel</button>
        {{else}}
        <input type="checkbox" {{if .Completed}}checked{{end}} lv-click="toggle" lv-value-id="{{.ID}}" />
        <span class="todo-text">{{.Title}}</span>
        <div class="todo-actions">
            <button class="edit" lv-click="edit" lv-value-id="{{.ID}}">Edit</button>
            <button class="delete" lv-click="delete" lv-value-id="{{.ID}}">Delete</button>
        </div>
        {{end}}
    </li>
    {{else}}
    <li class="todo-item" style="color: #999; justify-content: center;">
        No todos yet. Add one above!
    </li>
    {{end}}
</ul>

<div class="filters">
    <span>{{.Active}} item{{if ne .Active 1}}s{{end}} left</span>
    <div>
        <button class="{{if eq .Filter "all"}}active{{end}}" lv-click="filter" lv-value-filter="all">All</button>
        <button class="{{if eq .Filter "active"}}active{{end}}" lv-click="filter" lv-value-filter="active">Active</button>
        <button class="{{if eq .Filter "completed"}}active{{end}}" lv-click="filter" lv-value-filter="completed">Completed</button>
    </div>
    {{if gt .Completed 0}}
    <button lv-click="clear_completed">Clear completed ({{.Completed}})</button>
    {{end}}
</div>
`))
