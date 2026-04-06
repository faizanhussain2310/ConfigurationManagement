package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/faizanhussain/arbiter/pkg/engine"
)

// testStore creates a temporary SQLite store for testing.
func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() {
		s.Close()
		os.Remove(dbPath)
	})
	return s
}

func testRule(id, name, ruleType string) *engine.Rule {
	return &engine.Rule{
		ID:   id,
		Name: name,
		Type: ruleType,
		Tree: json.RawMessage(`{"value": true}`),
	}
}

func TestCreateAndGetRule(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := testRule("test_flag", "Test Flag", "feature_flag")
	if err := s.CreateRule(ctx, r, "test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.GetRule(ctx, "test_flag")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != "test_flag" {
		t.Errorf("expected ID 'test_flag', got %q", got.ID)
	}
	if got.Name != "Test Flag" {
		t.Errorf("expected name 'Test Flag', got %q", got.Name)
	}
	if got.Version != 1 {
		t.Errorf("expected version 1, got %d", got.Version)
	}
	if got.Status != "active" {
		t.Errorf("expected status 'active', got %q", got.Status)
	}
}

func TestCreateRuleDuplicateID(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := testRule("dup", "First", "feature_flag")
	if err := s.CreateRule(ctx, r, "test"); err != nil {
		t.Fatal(err)
	}
	r2 := testRule("dup", "Second", "feature_flag")
	if err := s.CreateRule(ctx, r2, "test"); err == nil {
		t.Error("expected error for duplicate ID")
	}
}

func TestListRulesPagination(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		r := testRule(
			string(rune('a'+i)),
			"Rule "+string(rune('A'+i)),
			"feature_flag",
		)
		if err := s.CreateRule(ctx, r, "test"); err != nil {
			t.Fatal(err)
		}
	}

	// Get first page
	rules, total, err := s.ListRules(ctx, 2, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}

	// Get second page
	rules, _, err = s.ListRules(ctx, 2, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}

	// Get last page
	rules, _, err = s.ListRules(ctx, 2, 4, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
}

func TestListRulesEmpty(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	rules, total, err := s.ListRules(ctx, 50, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
	if len(rules) != 0 {
		t.Errorf("expected empty slice, got %d rules", len(rules))
	}
}

func TestUpdateRuleCreatesNewVersion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := testRule("update_me", "Original", "feature_flag")
	if err := s.CreateRule(ctx, r, "test"); err != nil {
		t.Fatal(err)
	}

	r.Name = "Updated"
	r.Tree = json.RawMessage(`{"value": false}`)
	if err := s.UpdateRule(ctx, r, "test"); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetRule(ctx, "update_me")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 {
		t.Errorf("expected version 2, got %d", got.Version)
	}
	if got.Name != "Updated" {
		t.Errorf("expected name 'Updated', got %q", got.Name)
	}

	// Verify version history
	versions, err := s.ListVersions(ctx, "update_me")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
}

func TestDeleteRuleCascade(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := testRule("delete_me", "Delete Me", "feature_flag")
	if err := s.CreateRule(ctx, r, "test"); err != nil {
		t.Fatal(err)
	}

	// Add eval history
	if err := s.InsertEvalHistory(ctx, "delete_me",
		json.RawMessage(`{}`), json.RawMessage(`{"value":true}`)); err != nil {
		t.Fatal(err)
	}

	// Delete
	if err := s.DeleteRule(ctx, "delete_me"); err != nil {
		t.Fatal(err)
	}

	// Rule should be gone
	_, err := s.GetRule(ctx, "delete_me")
	if err == nil {
		t.Error("expected error after delete")
	}

	// Versions should be cascade-deleted
	versions, err := s.ListVersions(ctx, "delete_me")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions after cascade delete, got %d", len(versions))
	}

	// Eval history should be cascade-deleted
	entries, total, err := s.ListEvalHistory(ctx, "delete_me", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(entries) != 0 {
		t.Errorf("expected 0 eval history after cascade, got %d", total)
	}
}

func TestDeleteRuleNotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	err := s.DeleteRule(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error deleting nonexistent rule")
	}
}

func TestVersionHistory(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := testRule("versioned", "V1", "decision_tree")
	if err := s.CreateRule(ctx, r, "test"); err != nil {
		t.Fatal(err)
	}

	// Update twice
	r.Name = "V2"
	s.UpdateRule(ctx, r, "test")
	r.Name = "V3"
	s.UpdateRule(ctx, r, "test")

	versions, err := s.ListVersions(ctx, "versioned")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}
	// Versions are DESC, so first is v3
	if versions[0].Version != 3 {
		t.Errorf("expected first version to be 3, got %d", versions[0].Version)
	}
	if versions[0].Name != "V3" {
		t.Errorf("expected name 'V3', got %q", versions[0].Name)
	}
}

func TestGetVersion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := testRule("snap", "Snap V1", "feature_flag")
	r.Tree = json.RawMessage(`{"value": "original"}`)
	if err := s.CreateRule(ctx, r, "test"); err != nil {
		t.Fatal(err)
	}

	// Update
	r.Name = "Snap V2"
	r.Tree = json.RawMessage(`{"value": "updated"}`)
	s.UpdateRule(ctx, r, "test")

	// Get v1 snapshot
	v1, err := s.GetVersion(ctx, "snap", 1)
	if err != nil {
		t.Fatal(err)
	}
	if v1.Name != "Snap V1" {
		t.Errorf("expected v1 name 'Snap V1', got %q", v1.Name)
	}
	if string(v1.Tree) != `{"value": "original"}` {
		t.Errorf("expected v1 tree to be original, got %s", v1.Tree)
	}
}

func TestRollbackToVersion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := testRule("rollback_me", "Version 1", "feature_flag")
	r.Tree = json.RawMessage(`{"value": "v1"}`)
	if err := s.CreateRule(ctx, r, "test"); err != nil {
		t.Fatal(err)
	}

	// Update to v2
	r.Name = "Version 2"
	r.Tree = json.RawMessage(`{"value": "v2"}`)
	s.UpdateRule(ctx, r, "test")

	// Rollback to v1 (creates v3)
	rolled, err := s.RollbackToVersion(ctx, "rollback_me", 1, "test")
	if err != nil {
		t.Fatal(err)
	}
	if rolled.Version != 3 {
		t.Errorf("expected rollback to create v3, got v%d", rolled.Version)
	}
	if rolled.Name != "Version 1" {
		t.Errorf("expected rolled back name 'Version 1', got %q", rolled.Name)
	}

	// Verify in DB
	got, _ := s.GetRule(ctx, "rollback_me")
	if got.Version != 3 {
		t.Errorf("expected rule at v3, got v%d", got.Version)
	}

	// Verify 3 versions exist
	versions, _ := s.ListVersions(ctx, "rollback_me")
	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(versions))
	}
}

func TestRollbackNonexistentVersion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := testRule("rb_err", "Test", "feature_flag")
	s.CreateRule(ctx, r, "test")

	_, err := s.RollbackToVersion(ctx, "rb_err", 99, "test")
	if err == nil {
		t.Error("expected error rolling back to nonexistent version")
	}
}

func TestDuplicateRule(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := testRule("original", "Original Rule", "decision_tree")
	r.Description = "A test rule"
	r.Tree = json.RawMessage(`{"value": "hello"}`)
	if err := s.CreateRule(ctx, r, "test"); err != nil {
		t.Fatal(err)
	}

	dup, err := s.DuplicateRule(ctx, "original", "original-copy", "test")
	if err != nil {
		t.Fatal(err)
	}
	if dup.ID != "original-copy" {
		t.Errorf("expected ID 'original-copy', got %q", dup.ID)
	}
	if dup.Name != "Original Rule-copy" {
		t.Errorf("expected name 'Original Rule-copy', got %q", dup.Name)
	}
	if dup.Version != 1 {
		t.Errorf("expected duplicate at version 1, got %d", dup.Version)
	}
	if dup.Description != "A test rule" {
		t.Errorf("expected description preserved, got %q", dup.Description)
	}

	// Verify it's in the DB
	got, err := s.GetRule(ctx, "original-copy")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Tree) != `{"value": "hello"}` {
		t.Errorf("expected tree preserved, got %s", got.Tree)
	}
}

func TestDuplicateNonexistentRule(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.DuplicateRule(ctx, "ghost", "ghost-copy", "test")
	if err == nil {
		t.Error("expected error duplicating nonexistent rule")
	}
}

func TestEvalHistory(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := testRule("hist", "History Test", "feature_flag")
	s.CreateRule(ctx, r, "test")

	// Insert some history
	for i := 0; i < 5; i++ {
		s.InsertEvalHistory(ctx, "hist",
			json.RawMessage(`{"user":"test"}`),
			json.RawMessage(`{"value":true}`))
	}

	entries, total, err := s.ListEvalHistory(ctx, "hist", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 {
		t.Errorf("expected 5 entries, got %d", total)
	}
	if len(entries) != 5 {
		t.Errorf("expected 5 entries in slice, got %d", len(entries))
	}
	// Verify entries are ordered by created_at DESC (ties broken by insertion order)
	// All 5 inserts happen within the same second, so just verify we got them all
	for _, e := range entries {
		if e.RuleID != "hist" {
			t.Errorf("expected rule_id 'hist', got %q", e.RuleID)
		}
	}
}

func TestEvalHistoryPagination(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := testRule("hist_page", "Pagination", "feature_flag")
	s.CreateRule(ctx, r, "test")

	for i := 0; i < 10; i++ {
		s.InsertEvalHistory(ctx, "hist_page",
			json.RawMessage(`{}`), json.RawMessage(`{"value":true}`))
	}

	entries, total, err := s.ListEvalHistory(ctx, "hist_page", 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 10 {
		t.Errorf("expected total 10, got %d", total)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestMeta(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Set
	if err := s.SetMeta(ctx, "test_key", "test_value"); err != nil {
		t.Fatal(err)
	}

	// Get
	val, err := s.GetMeta(ctx, "test_key")
	if err != nil {
		t.Fatal(err)
	}
	if val != "test_value" {
		t.Errorf("expected 'test_value', got %q", val)
	}

	// Upsert
	s.SetMeta(ctx, "test_key", "updated")
	val, _ = s.GetMeta(ctx, "test_key")
	if val != "updated" {
		t.Errorf("expected 'updated' after upsert, got %q", val)
	}
}

func TestMetaNotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.GetMeta(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for missing meta key")
	}
}

func TestRuleExists(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	exists, err := s.RuleExists(ctx, "nope")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("expected false for nonexistent rule")
	}

	r := testRule("yep", "Exists", "feature_flag")
	s.CreateRule(ctx, r, "test")

	exists, err = s.RuleExists(ctx, "yep")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected true for existing rule")
	}
}

func TestImportRuleNew(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := testRule("imported", "Imported Rule", "feature_flag")
	if err := s.ImportRule(ctx, r, false, "test"); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetRule(ctx, "imported")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Imported Rule" {
		t.Errorf("expected name 'Imported Rule', got %q", got.Name)
	}
}

func TestImportRuleConflict(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := testRule("conflict", "Original", "feature_flag")
	s.CreateRule(ctx, r, "test")

	r2 := testRule("conflict", "Replacement", "feature_flag")
	err := s.ImportRule(ctx, r2, false, "test")
	if err == nil {
		t.Error("expected conflict error")
	}
}

func TestImportRuleForce(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := testRule("force_me", "Original", "feature_flag")
	s.CreateRule(ctx, r, "test")

	r2 := testRule("force_me", "Forced Update", "feature_flag")
	r2.Tree = json.RawMessage(`{"value": "new"}`)
	r2.Status = "active"
	if err := s.ImportRule(ctx, r2, true, "test"); err != nil {
		t.Fatal(err)
	}

	got, _ := s.GetRule(ctx, "force_me")
	if got.Name != "Forced Update" {
		t.Errorf("expected force-imported name, got %q", got.Name)
	}
	if got.Version != 2 {
		t.Errorf("expected version 2 after force import, got %d", got.Version)
	}
}

func TestDefaultValueRoundTrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := &engine.Rule{
		ID:           "with_default",
		Name:         "With Default",
		Type:         "feature_flag",
		Tree:         json.RawMessage(`{"value": true}`),
		DefaultValue: json.RawMessage(`"fallback"`),
	}
	if err := s.CreateRule(ctx, r, "test"); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetRule(ctx, "with_default")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.DefaultValue) != `"fallback"` {
		t.Errorf("expected default_value '\"fallback\"', got %q", string(got.DefaultValue))
	}
}

func TestNullDefaultValue(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := testRule("no_default", "No Default", "feature_flag")
	s.CreateRule(ctx, r, "test")

	got, _ := s.GetRule(ctx, "no_default")
	if got.DefaultValue != nil {
		t.Errorf("expected nil default_value, got %s", got.DefaultValue)
	}
}

func TestTreeRoundTrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	tree := `{"condition":{"field":"x","op":"eq","value":1},"then":{"value":"yes"},"else":{"value":"no"}}`
	r := &engine.Rule{
		ID:   "tree_rt",
		Name: "Tree Round Trip",
		Type: "decision_tree",
		Tree: json.RawMessage(tree),
	}
	if err := s.CreateRule(ctx, r, "test"); err != nil {
		t.Fatal(err)
	}

	got, _ := s.GetRule(ctx, "tree_rt")
	if string(got.Tree) != tree {
		t.Errorf("tree round-trip mismatch:\n  want: %s\n  got:  %s", tree, got.Tree)
	}
}
