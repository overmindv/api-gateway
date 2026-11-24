package graphql

import (
	"testing"

	"github.com/overmindv/api-gateway/internal/graphql/model"
)

// TestITCodeSubmissionPassesSQLiteLanguage проверяет, что новое значение enum
// доходит до tasks вместе с режимом graded.
func TestITCodeSubmissionPassesSQLiteLanguage(t *testing.T) {
	t.Parallel()

	stub := &tasksServiceStub{}
	root := &Resolver{Tasks: stub}
	mutation := &mutationResolver{Resolver: root}
	ctx := tasksContext(testUserID, []string{"student"})
	_, err := mutation.SubmitITTaskCode(ctx, "task-id", model.ITCodeSubmissionInput{
		TaskVersionID:  "version-id",
		IdempotencyKey: testKeyID,
		RunMode:        model.ITCodeRunModeGraded,
		Language:       model.ITProgrammingLanguageSqlite,
		SourceCode:     new("SELECT 1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if stub.codeInput.Language != "sqlite" || stub.codeInput.RunMode != runModeGraded || stub.codeInput.SourceCode == nil || *stub.codeInput.SourceCode != "SELECT 1" {
		t.Fatalf("unexpected code input: %+v", stub.codeInput)
	}
}
