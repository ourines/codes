package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMultiNotifier_Send(t *testing.T) {
	var called []string

	n1 := &mockNotifier{name: "a", sendFn: func(n Notification) error {
		called = append(called, "a")
		return nil
	}}
	n2 := &mockNotifier{name: "b", sendFn: func(n Notification) error {
		called = append(called, "b")
		return nil
	}}

	m := NewMultiNotifier(n1, n2)
	err := m.Send(Notification{Title: "test", Message: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(called) != 2 || called[0] != "a" || called[1] != "b" {
		t.Fatalf("expected both notifiers called, got: %v", called)
	}
}

func TestMultiNotifier_Name(t *testing.T) {
	m := NewMultiNotifier(
		&mockNotifier{name: "x"},
		&mockNotifier{name: "y"},
	)
	got := m.Name()
	want := "multi(x,y)"
	if got != want {
		t.Fatalf("Name() = %q, want %q", got, want)
	}
}

func TestWebhookNotifier_Slack(t *testing.T) {
	var received map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	wh := NewWebhookNotifier(srv.URL, "slack")
	err := wh.Send(Notification{Title: "task done", Message: "build passed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received["text"] != "task done: build passed" {
		t.Fatalf("unexpected payload: %v", received)
	}
}

func TestWebhookNotifier_Feishu(t *testing.T) {
	var received map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	wh := NewWebhookNotifier(srv.URL, "feishu")
	err := wh.Send(Notification{Title: "deploy", Message: "v1.0 released"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received["msg_type"] != "text" {
		t.Fatalf("expected msg_type=text, got: %v", received["msg_type"])
	}
	content, ok := received["content"].(map[string]any)
	if !ok {
		t.Fatalf("expected content map, got: %T", received["content"])
	}
	if content["text"] != "deploy: v1.0 released" {
		t.Fatalf("unexpected content text: %v", content["text"])
	}
}

func TestWebhookNotifier_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	wh := NewWebhookNotifier(srv.URL, "slack")
	err := wh.Send(Notification{Title: "test", Message: "msg"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestNewDesktopNotifier(t *testing.T) {
	n := NewDesktopNotifier()
	if n == nil {
		t.Fatal("NewDesktopNotifier returned nil")
	}
	if n.Name() == "" {
		t.Fatal("notifier Name() is empty")
	}
}

// mockNotifier is a test helper.
type mockNotifier struct {
	name   string
	sendFn func(Notification) error
}

func (m *mockNotifier) Send(n Notification) error {
	if m.sendFn != nil {
		return m.sendFn(n)
	}
	return nil
}

func (m *mockNotifier) Name() string { return m.name }
