package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/domain"
)

func TestCommitCASAndTamperDetection(t *testing.T) {
	dir := t.TempDir()
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, err := domain.NewCase("case-store", "演练", "A", "coord", []string{"observer"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Commit(CommitInput{Case: c, ExpectedRevision: 0, Event: Event{Type: "case.created", ActorID: "coord"}}); err != nil {
		t.Fatal(err)
	}
	stale, _ := domain.NewCase("case-store", "覆盖", "B", "coord", []string{"observer"}, time.Now())
	if err := repo.Commit(CommitInput{Case: stale, ExpectedRevision: 0, Event: Event{Type: "case.created", ActorID: "coord"}}); domain.ErrorCode(err) != domain.CodeConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "events.log")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	payload[len(payload)-1] ^= 0xff
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	damaged, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer damaged.Close()
	if damaged.Integrity().Healthy {
		t.Fatal("tampered event log reported healthy")
	}
	loaded, err := damaged.Load("case-store")
	if err != nil {
		t.Fatal(err)
	}
	if err := damaged.Commit(CommitInput{Case: loaded, ExpectedRevision: loaded.Revision, Event: Event{Type: "test", ActorID: "coord"}}); domain.ErrorCode(err) != domain.CodeIntegrity {
		t.Fatalf("expected write gate, got %v", err)
	}
}

func TestPersistentIdempotencyIndex(t *testing.T) {
	dir := t.TempDir()
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	result := StoredResult{RequestID: "req-1", Fingerprint: "abc", HTTPStatus: 422, Body: []byte(`{"error":{"code":"validation_error","message":"无效"}}`), CreatedAt: time.Now()}
	if err := repo.SaveResult(result); err != nil {
		t.Fatal(err)
	}
	if err := repo.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.LookupResult("req-1", "abc")
	if err != nil || got == nil || got.HTTPStatus != 422 {
		t.Fatalf("result not persisted: %#v %v", got, err)
	}
	if _, err := reopened.LookupResult("req-1", "different"); domain.ErrorCode(err) != domain.CodeIdempotency {
		t.Fatalf("expected key reuse error, got %v", err)
	}
}
