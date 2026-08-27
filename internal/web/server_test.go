package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/application"
	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/store"
)

func testServer(t *testing.T) (*httptest.Server, *store.Repository) {
	t.Helper()
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewServer(application.NewService(repo)).Handler())
	return server, repo
}

func TestWorkspaceAndCreateAPI(t *testing.T) {
	server, repo := testServer(t)
	defer server.Close()
	defer repo.Close()
	response, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 4096)
	n, _ := response.Body.Read(payload)
	response.Body.Close()
	if response.StatusCode != 200 || !strings.Contains(string(payload[:n]), "化学品泄漏演练就绪门禁") {
		t.Fatalf("workspace unavailable: %d", response.StatusCode)
	}
	if response.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("security headers missing")
	}
	body, _ := json.Marshal(map[string]any{"request_id": "web-create", "expected_revision": 0, "actor_id": "coord", "title": "演练", "building": "A", "coordinator_id": "coord", "observer_ids": []string{"observer"}})
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/cases", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", response.StatusCode)
	}
	var envelope struct {
		Data struct {
			Status   string `json:"status"`
			Revision int64  `json:"revision"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Status != "draft" || envelope.Data.Revision != 1 {
		t.Fatalf("unexpected create response: %#v", envelope.Data)
	}
}

func TestRejectsWrongContentTypeAndReportsReadiness(t *testing.T) {
	server, repo := testServer(t)
	defer server.Close()
	defer repo.Close()
	response, err := http.Post(server.URL+"/api/cases", "text/plain", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d", response.StatusCode)
	}
	response, err = http.Get(server.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("ready status = %d", response.StatusCode)
	}
}
