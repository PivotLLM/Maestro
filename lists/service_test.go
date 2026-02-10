/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package lists

import (
	"embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/logging"
)

//go:embed testdata
var testFS embed.FS

func setupTestService(t *testing.T) (*Service, string) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "lists_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	projectsDir := filepath.Join(tempDir, "projects")
	playbooksDir := filepath.Join(tempDir, "playbooks")

	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create projects dir: %v", err)
	}
	if err := os.MkdirAll(playbooksDir, 0755); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create playbooks dir: %v", err)
	}

	logFile := filepath.Join(tempDir, "test.log")
	logger, err := logging.New(logFile)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create logger: %v", err)
	}
	service := NewService(
		WithProjectsDir(projectsDir),
		WithPlaybooksDir(playbooksDir),
		WithEmbeddedFS(testFS),
		WithLogger(logger),
	)

	return service, tempDir
}

func createTestProject(t *testing.T, tempDir, projectName string) {
	t.Helper()
	projectDir := filepath.Join(tempDir, "projects", projectName)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}
}

func createTestPlaybook(t *testing.T, tempDir, playbookName string) {
	t.Helper()
	playbookDir := filepath.Join(tempDir, "playbooks", playbookName)
	if err := os.MkdirAll(playbookDir, 0755); err != nil {
		t.Fatalf("Failed to create playbook dir: %v", err)
	}
}

func TestListCreate(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	// Test creating a list
	err := service.Create(SourceProject, "test-project", "", "items.json", "Test List", "A test list", nil)
	if err != nil {
		t.Fatalf("Failed to create list: %v", err)
	}

	// Verify it was created
	list, err := service.Get(SourceProject, "test-project", "", "items.json")
	if err != nil {
		t.Fatalf("Failed to get list: %v", err)
	}

	if list.Name != "Test List" {
		t.Errorf("Expected name 'Test List', got '%s'", list.Name)
	}
	if list.Description != "A test list" {
		t.Errorf("Expected description 'A test list', got '%s'", list.Description)
	}
	if list.Version != global.ListSchemaVersion {
		t.Errorf("Expected version '%s', got '%s'", global.ListSchemaVersion, list.Version)
	}
}

func TestListCreateDuplicate(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	// Create first list
	err := service.Create(SourceProject, "test-project", "", "items.json", "Test List", "", nil)
	if err != nil {
		t.Fatalf("Failed to create first list: %v", err)
	}

	// Try to create duplicate
	err = service.Create(SourceProject, "test-project", "", "items.json", "Another List", "", nil)
	if err == nil {
		t.Error("Expected error when creating duplicate list")
	}
}

func TestListCreateWithItems(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	items := []global.ListItem{
		{ID: "item-1", Title: "First", Content: "First item"},
		{ID: "item-2", Title: "Second", Content: "Second item"},
	}

	err := service.Create(SourceProject, "test-project", "", "items.json", "Test List", "", items)
	if err != nil {
		t.Fatalf("Failed to create list with items: %v", err)
	}

	list, err := service.Get(SourceProject, "test-project", "", "items.json")
	if err != nil {
		t.Fatalf("Failed to get list: %v", err)
	}

	if len(list.Items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(list.Items))
	}
}

func TestListCreateWithDuplicateItemIDs(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	items := []global.ListItem{
		{ID: "item-1", Title: "First", Content: "First item"},
		{ID: "item-1", Title: "Duplicate", Content: "Duplicate item"},
	}

	err := service.Create(SourceProject, "test-project", "", "items.json", "Test List", "", items)
	if err == nil {
		t.Error("Expected error when creating list with duplicate item IDs")
	}
}

func TestListCreateInReference(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	err := service.Create(SourceReference, "", "", "items.json", "Test List", "", nil)
	if err == nil {
		t.Error("Expected error when creating list in reference domain")
	}
}

func TestListDelete(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	// Create a list
	err := service.Create(SourceProject, "test-project", "", "items.json", "Test List", "", nil)
	if err != nil {
		t.Fatalf("Failed to create list: %v", err)
	}

	// Delete it
	err = service.Delete(SourceProject, "test-project", "", "items.json")
	if err != nil {
		t.Fatalf("Failed to delete list: %v", err)
	}

	// Verify it's gone
	_, err = service.Get(SourceProject, "test-project", "", "items.json")
	if err == nil {
		t.Error("Expected error when getting deleted list")
	}
}

func TestListRename(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	// Create a list
	err := service.Create(SourceProject, "test-project", "", "old.json", "Test List", "", nil)
	if err != nil {
		t.Fatalf("Failed to create list: %v", err)
	}

	// Rename it
	err = service.Rename(SourceProject, "test-project", "", "old.json", "new.json")
	if err != nil {
		t.Fatalf("Failed to rename list: %v", err)
	}

	// Verify old name is gone
	_, err = service.Get(SourceProject, "test-project", "", "old.json")
	if err == nil {
		t.Error("Expected error when getting old list name")
	}

	// Verify new name exists
	list, err := service.Get(SourceProject, "test-project", "", "new.json")
	if err != nil {
		t.Fatalf("Failed to get renamed list: %v", err)
	}
	if list.Name != "Test List" {
		t.Errorf("Expected name 'Test List', got '%s'", list.Name)
	}
}

func TestItemAdd(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	// Create a list
	err := service.Create(SourceProject, "test-project", "", "items.json", "Test List", "", nil)
	if err != nil {
		t.Fatalf("Failed to create list: %v", err)
	}

	// Add an item - ID will be auto-generated
	item := &global.ListItem{
		Title:     "Test Item",
		Content:   "Test content",
		SourceDoc: "doc.md",
		Section:   "Section 1",
		Tags:      []string{"tag1", "tag2"},
	}
	assignedID, err := service.AddItem(SourceProject, "test-project", "", "items.json", item)
	if err != nil {
		t.Fatalf("Failed to add item: %v", err)
	}

	// Verify auto-generated ID format
	if assignedID != "item-001" {
		t.Errorf("Expected auto-generated ID 'item-001', got '%s'", assignedID)
	}

	// Verify item was added
	list, err := service.Get(SourceProject, "test-project", "", "items.json")
	if err != nil {
		t.Fatalf("Failed to get list: %v", err)
	}

	if len(list.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(list.Items))
	}

	if list.Items[0].ID != "item-001" {
		t.Errorf("Expected item ID 'item-001', got '%s'", list.Items[0].ID)
	}
	if list.Items[0].Content != "Test content" {
		t.Errorf("Expected content 'Test content', got '%s'", list.Items[0].Content)
	}
}

func TestItemAddAutoIncrementID(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	// Create a list
	err := service.Create(SourceProject, "test-project", "", "items.json", "Test List", "", nil)
	if err != nil {
		t.Fatalf("Failed to create list: %v", err)
	}

	// Add first item
	item1 := &global.ListItem{Title: "First Item", Content: "First"}
	id1, err := service.AddItem(SourceProject, "test-project", "", "items.json", item1)
	if err != nil {
		t.Fatalf("Failed to add first item: %v", err)
	}
	if id1 != "item-001" {
		t.Errorf("Expected first item ID 'item-001', got '%s'", id1)
	}

	// Add second item
	item2 := &global.ListItem{Title: "Second Item", Content: "Second"}
	id2, err := service.AddItem(SourceProject, "test-project", "", "items.json", item2)
	if err != nil {
		t.Fatalf("Failed to add second item: %v", err)
	}
	if id2 != "item-002" {
		t.Errorf("Expected second item ID 'item-002', got '%s'", id2)
	}

	// Add third item
	item3 := &global.ListItem{Title: "Third Item", Content: "Third"}
	id3, err := service.AddItem(SourceProject, "test-project", "", "items.json", item3)
	if err != nil {
		t.Fatalf("Failed to add third item: %v", err)
	}
	if id3 != "item-003" {
		t.Errorf("Expected third item ID 'item-003', got '%s'", id3)
	}
}

func TestItemAddIgnoresProvidedID(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	// Create a list
	err := service.Create(SourceProject, "test-project", "", "items.json", "Test List", "", nil)
	if err != nil {
		t.Fatalf("Failed to create list: %v", err)
	}

	// Add item with a custom ID - should be ignored
	item := &global.ListItem{
		ID:      "CUSTOM-ID",
		Title:   "Test Item",
		Content: "Test content",
	}
	assignedID, err := service.AddItem(SourceProject, "test-project", "", "items.json", item)
	if err != nil {
		t.Fatalf("Failed to add item: %v", err)
	}

	// Verify custom ID was ignored and auto-generated ID was used
	if assignedID != "item-001" {
		t.Errorf("Expected auto-generated ID 'item-001', got '%s' (custom ID should be ignored)", assignedID)
	}

	// Verify CUSTOM-ID does not exist
	_, err = service.GetItem(SourceProject, "test-project", "", "items.json", "CUSTOM-ID")
	if err == nil {
		t.Error("Expected error when getting item by custom ID (should not exist)")
	}

	// Verify item-001 exists
	retrievedItem, err := service.GetItem(SourceProject, "test-project", "", "items.json", "item-001")
	if err != nil {
		t.Fatalf("Failed to get item by auto-generated ID: %v", err)
	}
	if retrievedItem.Title != "Test Item" {
		t.Errorf("Expected title 'Test Item', got '%s'", retrievedItem.Title)
	}
}

func TestItemUpdate(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	// Create list with item
	items := []global.ListItem{{ID: "item-1", Title: "Original Title", Content: "Original"}}
	err := service.Create(SourceProject, "test-project", "", "items.json", "Test List", "", items)
	if err != nil {
		t.Fatalf("Failed to create list: %v", err)
	}

	// Update item
	newContent := "Updated content"
	err = service.UpdateItem(SourceProject, "test-project", "", "items.json", "item-1", nil, &newContent, nil, nil, nil, false, nil)
	if err != nil {
		t.Fatalf("Failed to update item: %v", err)
	}

	// Verify update
	item, err := service.GetItem(SourceProject, "test-project", "", "items.json", "item-1")
	if err != nil {
		t.Fatalf("Failed to get item: %v", err)
	}

	if item.Content != "Updated content" {
		t.Errorf("Expected content 'Updated content', got '%s'", item.Content)
	}
}

func TestItemRemove(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	// Create list with items
	items := []global.ListItem{
		{ID: "item-1", Title: "First", Content: "First"},
		{ID: "item-2", Title: "Second", Content: "Second"},
	}
	err := service.Create(SourceProject, "test-project", "", "items.json", "Test List", "", items)
	if err != nil {
		t.Fatalf("Failed to create list: %v", err)
	}

	// Remove an item
	err = service.RemoveItem(SourceProject, "test-project", "", "items.json", "item-1")
	if err != nil {
		t.Fatalf("Failed to remove item: %v", err)
	}

	// Verify removal
	list, err := service.Get(SourceProject, "test-project", "", "items.json")
	if err != nil {
		t.Fatalf("Failed to get list: %v", err)
	}

	if len(list.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(list.Items))
	}
	if list.Items[0].ID != "item-2" {
		t.Errorf("Expected remaining item 'item-2', got '%s'", list.Items[0].ID)
	}
}

func TestItemRename(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	// Create list with item
	items := []global.ListItem{{ID: "old-id", Title: "Test", Content: "Test"}}
	err := service.Create(SourceProject, "test-project", "", "items.json", "Test List", "", items)
	if err != nil {
		t.Fatalf("Failed to create list: %v", err)
	}

	// Rename item
	err = service.RenameItem(SourceProject, "test-project", "", "items.json", "old-id", "new-id")
	if err != nil {
		t.Fatalf("Failed to rename item: %v", err)
	}

	// Verify rename
	_, err = service.GetItem(SourceProject, "test-project", "", "items.json", "old-id")
	if err == nil {
		t.Error("Expected error when getting old item ID")
	}

	item, err := service.GetItem(SourceProject, "test-project", "", "items.json", "new-id")
	if err != nil {
		t.Fatalf("Failed to get renamed item: %v", err)
	}
	if item.Content != "Test" {
		t.Errorf("Expected content 'Test', got '%s'", item.Content)
	}
}

func TestItemSearch(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	// Create list with items
	items := []global.ListItem{
		{ID: "req-001", Title: "Auth Required", Content: "User authentication required", SourceDoc: "doc1.md", Tags: []string{"security", "auth"}},
		{ID: "req-002", Title: "Password Length", Content: "Password must be 8 chars", SourceDoc: "doc1.md", Tags: []string{"security"}},
		{ID: "req-003", Title: "Data Export", Content: "Data export feature", SourceDoc: "doc2.md", Tags: []string{"feature"}},
	}
	err := service.Create(SourceProject, "test-project", "", "items.json", "Test List", "", items)
	if err != nil {
		t.Fatalf("Failed to create list: %v", err)
	}

	// Search by query (content)
	result, err := service.SearchItems(SourceProject, "test-project", "", "items.json", "password", "", "", nil, "", 0, 50)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}
	if result.TotalCount != 1 {
		t.Errorf("Expected 1 match for 'password', got %d", result.TotalCount)
	}

	// Search by query (ID)
	result, err = service.SearchItems(SourceProject, "test-project", "", "items.json", "req-001", "", "", nil, "", 0, 50)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}
	if result.TotalCount != 1 {
		t.Errorf("Expected 1 match for 'req-001', got %d", result.TotalCount)
	}

	// Search by source_doc
	result, err = service.SearchItems(SourceProject, "test-project", "", "items.json", "", "doc1.md", "", nil, "", 0, 50)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}
	if result.TotalCount != 2 {
		t.Errorf("Expected 2 matches for source_doc 'doc1.md', got %d", result.TotalCount)
	}

	// Search by tags (AND logic)
	result, err = service.SearchItems(SourceProject, "test-project", "", "items.json", "", "", "", []string{"security", "auth"}, "", 0, 50)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}
	if result.TotalCount != 1 {
		t.Errorf("Expected 1 match for tags ['security', 'auth'], got %d", result.TotalCount)
	}
}

func TestListInPlaybook(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestPlaybook(t, tempDir, "test-playbook")

	// Create a list in playbook
	err := service.Create(SourcePlaybook, "", "test-playbook", "items.json", "Playbook List", "", nil)
	if err != nil {
		t.Fatalf("Failed to create list in playbook: %v", err)
	}

	// Get the list
	list, err := service.Get(SourcePlaybook, "", "test-playbook", "items.json")
	if err != nil {
		t.Fatalf("Failed to get list from playbook: %v", err)
	}

	if list.Name != "Playbook List" {
		t.Errorf("Expected name 'Playbook List', got '%s'", list.Name)
	}
}

func TestListList(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	// Create multiple lists
	err := service.Create(SourceProject, "test-project", "", "list1.json", "List 1", "", nil)
	if err != nil {
		t.Fatalf("Failed to create list1: %v", err)
	}
	err = service.Create(SourceProject, "test-project", "", "list2.json", "List 2", "", nil)
	if err != nil {
		t.Fatalf("Failed to create list2: %v", err)
	}

	// List all
	result, err := service.List(SourceProject, "test-project", "", 0, 50)
	if err != nil {
		t.Fatalf("Failed to list: %v", err)
	}

	if result.TotalCount != 2 {
		t.Errorf("Expected 2 lists, got %d", result.TotalCount)
	}
}

func TestGetSummary(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	// Create list with items
	items := []global.ListItem{
		{ID: "item-1", Title: "Short", Content: "Short content"},
		{ID: "item-2", Title: "Long", Content: "This is a much longer piece of content that should be truncated when displayed in the summary view because it exceeds 100 characters"},
	}
	err := service.Create(SourceProject, "test-project", "", "items.json", "Test List", "", items)
	if err != nil {
		t.Fatalf("Failed to create list: %v", err)
	}

	// Get summary
	result, err := service.GetSummary(SourceProject, "test-project", "", "items.json", "", 0, 50)
	if err != nil {
		t.Fatalf("Failed to get summary: %v", err)
	}

	if result.ItemCount != 2 {
		t.Errorf("Expected 2 items, got %d", result.ItemCount)
	}

	// Check truncation
	if len(result.Items[1].Content) > 103 { // 100 + "..."
		t.Errorf("Expected truncated content, got length %d", len(result.Items[1].Content))
	}
}

func TestListNameValidation(t *testing.T) {
	service, tempDir := setupTestService(t)
	defer os.RemoveAll(tempDir)

	createTestProject(t, tempDir, "test-project")

	// Test invalid list names (path traversal attempts and empty)
	invalidNames := []string{
		"",
		"../traversal",
		"sub/path",
		"back\\slash",
	}

	for _, name := range invalidNames {
		err := service.Create(SourceProject, "test-project", "", name, "Test", "", nil)
		if err == nil {
			t.Errorf("Expected error for list name '%s'", name)
		}
	}

	// Test valid list names - these should work (extension is auto-added)
	validNames := []string{
		"mylist",
		"my-list",
		"my_list",
		"another.json", // .json is stripped and re-added, so this works
	}

	for i, name := range validNames {
		err := service.Create(SourceProject, "test-project", "", name, "Test "+name, "", nil)
		if err != nil {
			t.Errorf("Unexpected error for list name '%s' (index %d): %v", name, i, err)
		}
	}
}
