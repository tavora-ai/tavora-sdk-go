package store

import "testing"

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateAndListLists(t *testing.T) {
	s := newTestStore(t)
	l, err := s.CreateList("Germany", "cities to visit")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if l.ID == "" || l.Name != "Germany" {
		t.Fatalf("unexpected list: %+v", l)
	}
	lists, err := s.ListLists()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lists) != 1 || lists[0].ID != l.ID {
		t.Fatalf("want 1 list with id=%s, got %+v", l.ID, lists)
	}
	if lists[0].TaskCount != 0 {
		t.Fatalf("want 0 tasks, got %d", lists[0].TaskCount)
	}
}

func TestAddListAndCompleteTask(t *testing.T) {
	s := newTestStore(t)
	l, _ := s.CreateList("Groceries", "")
	tk, err := s.AddTask(l.ID, "Milk", "", "")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if tk.Done {
		t.Fatalf("new task should not be done")
	}
	tasks, err := s.ListTasks(l.ID)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(tasks))
	}
	done, err := s.CompleteTask(tk.ID)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if !done.Done {
		t.Fatalf("task should be done")
	}
	lists, _ := s.ListLists()
	if lists[0].TaskCount != 1 {
		t.Fatalf("want 1 task count, got %d", lists[0].TaskCount)
	}
}

func TestAddTaskUnknownList(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.AddTask("no-such-list", "x", "", ""); err == nil {
		t.Fatalf("expected error for unknown list")
	}
}

func TestDeleteListCascades(t *testing.T) {
	s := newTestStore(t)
	l, _ := s.CreateList("L", "")
	s.AddTask(l.ID, "a", "", "")
	s.AddTask(l.ID, "b", "", "")
	if err := s.DeleteList(l.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	tasks, _ := s.ListTasks(l.ID)
	if len(tasks) != 0 {
		t.Fatalf("want 0 tasks after list delete, got %d", len(tasks))
	}
}
