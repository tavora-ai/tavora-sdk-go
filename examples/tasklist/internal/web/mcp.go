package web

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tavora-ai/tavora-sdk-go/examples/tasklist/internal/store"
)

// buildMCPServer builds the MCP server that exposes the six task-list
// tools to agents. Input/output structs are defined per tool so the SDK
// can auto-derive the JSON Schema from Go types.
func buildMCPServer(st *store.Store) *mcp.Server {
	srv := mcp.NewServer(
		&mcp.Implementation{Name: "tasklist", Version: "0.1.0"},
		nil,
	)

	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "create_task_list",
			Description: "Create a new task list with a name and optional description.",
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, in createTaskListIn) (*mcp.CallToolResult, createTaskListOut, error) {
			l, err := st.CreateList(in.Name, in.Description)
			if err != nil {
				return nil, createTaskListOut{}, err
			}
			return nil, createTaskListOut{ListID: l.ID, Name: l.Name}, nil
		},
	)

	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "list_task_lists",
			Description: "List all existing task lists with their IDs, names, and task counts.",
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ listTaskListsIn) (*mcp.CallToolResult, listTaskListsOut, error) {
			lists, err := st.ListLists()
			if err != nil {
				return nil, listTaskListsOut{}, err
			}
			out := listTaskListsOut{Lists: make([]listRef, 0, len(lists))}
			for _, l := range lists {
				out.Lists = append(out.Lists, listRef{ID: l.ID, Name: l.Name, TaskCount: l.TaskCount})
			}
			return nil, out, nil
		},
	)

	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "delete_task_list",
			Description: "Delete a task list and all its tasks by list_id.",
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, in deleteTaskListIn) (*mcp.CallToolResult, deleteTaskListOut, error) {
			if err := st.DeleteList(in.ListID); err != nil {
				return nil, deleteTaskListOut{}, err
			}
			return nil, deleteTaskListOut{Deleted: true}, nil
		},
	)

	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "add_task",
			Description: "Add a task to an existing task list. Only list_id and title are required.",
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, in addTaskIn) (*mcp.CallToolResult, addTaskOut, error) {
			t, err := st.AddTask(in.ListID, in.Title, in.Notes, in.DueDate)
			if err != nil {
				return nil, addTaskOut{}, err
			}
			return nil, addTaskOut{TaskID: t.ID, Title: t.Title}, nil
		},
	)

	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "list_tasks",
			Description: "List tasks in the given task list.",
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listTasksIn) (*mcp.CallToolResult, listTasksOut, error) {
			tasks, err := st.ListTasks(in.ListID)
			if err != nil {
				return nil, listTasksOut{}, err
			}
			out := listTasksOut{Tasks: make([]taskRef, 0, len(tasks))}
			for _, t := range tasks {
				out.Tasks = append(out.Tasks, taskRef{ID: t.ID, Title: t.Title, Done: t.Done})
			}
			return nil, out, nil
		},
	)

	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "complete_task",
			Description: "Mark a task as done by task_id.",
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, in completeTaskIn) (*mcp.CallToolResult, completeTaskOut, error) {
			t, err := st.CompleteTask(in.TaskID)
			if err != nil {
				return nil, completeTaskOut{}, err
			}
			return nil, completeTaskOut{TaskID: t.ID, Done: t.Done}, nil
		},
	)

	return srv
}

// mcpAuthHandler wraps the MCP streamable HTTP handler with a shared-secret
// Bearer gate so random callers can't hit /mcp. Tavora's MCP client is
// configured with the same token via auth_config at registration time.
func mcpAuthHandler(secret string, inner http.Handler) http.Handler {
	want := "Bearer " + secret
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != want {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		inner.ServeHTTP(w, r)
	})
}

// --- tool input/output shapes ---

type createTaskListIn struct {
	Name        string `json:"name" jsonschema:"the name of the task list"`
	Description string `json:"description,omitempty" jsonschema:"optional longer description"`
}
type createTaskListOut struct {
	ListID string `json:"list_id"`
	Name   string `json:"name"`
}

type listTaskListsIn struct{}
type listRef struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	TaskCount int    `json:"task_count"`
}
type listTaskListsOut struct {
	Lists []listRef `json:"lists"`
}

type deleteTaskListIn struct {
	ListID string `json:"list_id" jsonschema:"ID of the list to delete"`
}
type deleteTaskListOut struct {
	Deleted bool `json:"deleted"`
}

type addTaskIn struct {
	ListID  string `json:"list_id"  jsonschema:"ID of the list to add the task to"`
	Title   string `json:"title"    jsonschema:"short title of the task"`
	Notes   string `json:"notes,omitempty"    jsonschema:"optional longer notes"`
	DueDate string `json:"due_date,omitempty" jsonschema:"optional due date as a free-form string"`
}
type addTaskOut struct {
	TaskID string `json:"task_id"`
	Title  string `json:"title"`
}

type listTasksIn struct {
	ListID string `json:"list_id" jsonschema:"ID of the list to read"`
}
type taskRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}
type listTasksOut struct {
	Tasks []taskRef `json:"tasks"`
}

type completeTaskIn struct {
	TaskID string `json:"task_id" jsonschema:"ID of the task to complete"`
}
type completeTaskOut struct {
	TaskID string `json:"task_id"`
	Done   bool   `json:"done"`
}
