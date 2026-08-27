package application

import (
	"testing"

	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/domain"
	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/store"
)

func TestCreateIdempotencyAndRevisionConflict(t *testing.T) {
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := NewService(repo)
	command := CreateCaseCommand{CommandMeta: CommandMeta{RequestID: "create-1", ActorID: "coord", ExpectedRevision: 0}, Title: "演练", Building: "A", CoordinatorID: "coord", ObserverIDs: []string{"observer"}}
	first, err := service.CreateCase(command)
	if err != nil || first.Error != nil {
		t.Fatalf("create: %#v %v", first, err)
	}
	replay, err := service.CreateCase(command)
	if err != nil || !replay.Replayed || replay.Case.CaseID != first.Case.CaseID {
		t.Fatalf("replay failed: %#v %v", replay, err)
	}
	command.Title = "不同演练"
	reused, err := service.CreateCase(command)
	if err != nil || reused.Error == nil || reused.Error.Code != domain.CodeIdempotency {
		t.Fatalf("key reuse accepted: %#v %v", reused, err)
	}
	freeze := FreezeBaselineCommand{CommandMeta: CommandMeta{RequestID: "freeze-1", ActorID: "coord", ExpectedRevision: 0}, CaseID: first.Case.CaseID, Baseline: domain.BaselineInput{}}
	conflict, err := service.FreezeBaseline(freeze)
	if err != nil || conflict.Error == nil || conflict.Error.Code != domain.CodeConflict {
		t.Fatalf("stale revision accepted: %#v %v", conflict, err)
	}
	again, err := service.FreezeBaseline(freeze)
	if err != nil || !again.Replayed || again.Error.Message != conflict.Error.Message {
		t.Fatalf("business failure not replayed: %#v %v", again, err)
	}
}
