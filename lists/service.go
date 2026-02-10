/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

// Package lists provides management of structured list files across all domains.
// Lists are JSON files containing arrays of items with validated schemas.
package lists

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/logging"
)

// Source domain constants
const (
	SourceProject   = "project"
	SourcePlaybook  = "playbook"
	SourceReference = "reference"
)

// Service provides list operations across all domains.
type Service struct {
	projectsDir  string   // Base directory for projects
	playbooksDir string   // Base directory for playbooks
	referenceFS  embed.FS // Embedded reference filesystem
	logger       *logging.Logger
	pathMutex    sync.Map // per-path locking
}

// Option is a functional option for configuring Service
type Option func(*Service)

// WithProjectsDir sets the projects directory
func WithProjectsDir(dir string) Option {
	return func(s *Service) {
		s.projectsDir = dir
	}
}

// WithPlaybooksDir sets the playbooks directory
func WithPlaybooksDir(dir string) Option {
	return func(s *Service) {
		s.playbooksDir = dir
	}
}

// WithEmbeddedFS sets the embedded reference filesystem
func WithEmbeddedFS(efs embed.FS) Option {
	return func(s *Service) {
		s.referenceFS = efs
	}
}

// WithLogger sets the logger for the service
func WithLogger(logger *logging.Logger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

// NewService creates a new lists service with functional options.
func NewService(opts ...Option) *Service {
	s := &Service{
		pathMutex: sync.Map{},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// getPathMutex gets or creates a mutex for a specific path.
func (s *Service) getPathMutex(path string) *sync.Mutex {
	value, _ := s.pathMutex.LoadOrStore(path, &sync.Mutex{})
	return value.(*sync.Mutex)
}

// isWritable returns true if the source domain allows write operations.
// Empty string defaults to project, which is writable.
func isWritable(source string) bool {
	return source == SourceProject || source == SourcePlaybook || source == ""
}

// resolveListDir returns the lists directory path for the given source.
func (s *Service) resolveListDir(source, project, playbook string) (string, error) {
	switch source {
	case SourceProject, "":
		if project == "" {
			return "", fmt.Errorf("project is required when source is 'project'")
		}
		return filepath.Join(s.projectsDir, project, global.ListsDir), nil

	case SourcePlaybook:
		if playbook == "" {
			return "", fmt.Errorf("playbook is required when source is 'playbook'")
		}
		return filepath.Join(s.playbooksDir, playbook, global.ListsDir), nil

	case SourceReference:
		// Reference uses embedded FS, return the path prefix
		return filepath.Join("reference", global.ListsDir), nil

	default:
		return "", fmt.Errorf("invalid source: %s (must be 'project', 'playbook', or 'reference')", source)
	}
}

// normalizeListName validates and normalizes a list name to a filename.
// The name should not include .json extension - it will be added automatically.
// Returns the normalized filename (with .json extension).
func normalizeListName(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("list name cannot be empty")
	}
	// Check for path traversal
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", fmt.Errorf("list name cannot contain path separators or '..'")
	}
	// Strip .json if provided (for backwards compatibility), then add it
	name = strings.TrimSuffix(name, ".json")
	return name + ".json", nil
}

// validateItem validates a list item.
func validateItem(item *global.ListItem) error {
	// ID is optional - will be auto-generated if not provided
	if item.Title == "" {
		return fmt.Errorf("item title is required")
	}
	if item.Content == "" {
		return fmt.Errorf("item content is required")
	}
	return nil
}

// generateItemID generates a unique item ID based on existing items in the list.
// Format: item-001, item-002, etc.
func generateItemID(existingItems []global.ListItem) string {
	maxNum := 0
	for _, item := range existingItems {
		// Try to parse existing auto-generated IDs
		var num int
		if _, err := fmt.Sscanf(item.ID, "item-%d", &num); err == nil {
			if num > maxNum {
				maxNum = num
			}
		}
	}
	return fmt.Sprintf("item-%03d", maxNum+1)
}

// loadListFromFS loads a list from the embedded reference filesystem.
func (s *Service) loadListFromFS(path string) (*global.List, error) {
	data, err := s.referenceFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("list not found: %s", path)
	}

	var list global.List
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("failed to parse list: %w", err)
	}

	return &list, nil
}

// loadList loads a list from disk or embedded FS.
// The listName parameter should be the list name without .json extension.
func (s *Service) loadList(source, project, playbook, listName string) (*global.List, string, error) {
	filename, err := normalizeListName(listName)
	if err != nil {
		return nil, "", err
	}

	listDir, err := s.resolveListDir(source, project, playbook)
	if err != nil {
		return nil, "", err
	}

	if source == SourceReference {
		// Load from embedded FS
		path := filepath.ToSlash(filepath.Join(listDir, filename))
		list, err := s.loadListFromFS(path)
		if err != nil {
			return nil, "", err
		}
		return list, path, nil
	}

	// Load from disk
	filePath := filepath.Join(listDir, filename)

	mutex := s.getPathMutex(filePath)
	mutex.Lock()
	defer mutex.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("list not found: %s", filename)
		}
		return nil, "", fmt.Errorf("failed to read list: %w", err)
	}

	var list global.List
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, "", fmt.Errorf("failed to parse list: %w", err)
	}

	return &list, filePath, nil
}

// saveList saves a list to disk (must not be called for reference source).
func (s *Service) saveList(filePath string, list *global.List) error {
	list.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal list: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create list directory: %w", err)
	}

	// Atomic write
	if err := global.AtomicWrite(filePath, data); err != nil {
		return fmt.Errorf("failed to write list: %w", err)
	}

	return nil
}

// List returns all lists in the specified source.
func (s *Service) List(source, project, playbook string, offset, limit int) (*global.ListListResponse, error) {
	if limit <= 0 {
		limit = global.DefaultLimit
	}

	listDir, err := s.resolveListDir(source, project, playbook)
	if err != nil {
		return nil, err
	}

	var summaries []global.ListSummary

	if source == SourceReference {
		// List from embedded FS
		err := fs.WalkDir(s.referenceFS, listDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // Skip errors
			}
			if d.IsDir() || !strings.HasSuffix(path, ".json") {
				return nil
			}

			list, loadErr := s.loadListFromFS(path)
			if loadErr != nil {
				return nil
			}

			summaries = append(summaries, global.ListSummary{
				Filename:    filepath.Base(path),
				Name:        list.Name,
				Description: list.Description,
				ItemCount:   len(list.Items),
				UpdatedAt:   list.UpdatedAt,
			})
			return nil
		})
		if err != nil {
			s.logger.Warnf("Error walking reference lists: %v", err)
		}
	} else {
		// List from disk
		if !global.DirExists(listDir) {
			// No lists directory yet, return empty
			return &global.ListListResponse{
				Lists:         []global.ListSummary{},
				TotalCount:    0,
				ReturnedCount: 0,
				Offset:        offset,
			}, nil
		}

		entries, err := os.ReadDir(listDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read lists directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}

			filePath := filepath.Join(listDir, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			var list global.List
			if err := json.Unmarshal(data, &list); err != nil {
				continue
			}

			summaries = append(summaries, global.ListSummary{
				Filename:    entry.Name(),
				Name:        list.Name,
				Description: list.Description,
				ItemCount:   len(list.Items),
				UpdatedAt:   list.UpdatedAt,
			})
		}
	}

	// Apply pagination
	total := len(summaries)
	if offset >= total {
		return &global.ListListResponse{
			Lists:         []global.ListSummary{},
			TotalCount:    total,
			ReturnedCount: 0,
			Offset:        offset,
		}, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	result := summaries[offset:end]

	s.logger.Debugf("Listed %d lists (total: %d)", len(result), total)
	return &global.ListListResponse{
		Lists:         result,
		TotalCount:    total,
		ReturnedCount: len(result),
		Offset:        offset,
	}, nil
}

// Get returns the full contents of a list.
// The listName parameter should be the list name without .json extension.
func (s *Service) Get(source, project, playbook, listName string) (*global.List, error) {
	list, _, err := s.loadList(source, project, playbook, listName)
	if err != nil {
		return nil, err
	}

	s.logger.Debugf("Retrieved list: %s", listName)
	return list, nil
}

// GetSummary returns a summary of a list with paginated items.
// The listName parameter should be the list name without .json extension.
// The completeFilter parameter is only used for project lists: "true", "false", or "" (no filter).
func (s *Service) GetSummary(source, project, playbook, listName string, completeFilter string, offset, limit int) (*global.ListGetSummaryResponse, error) {
	if limit <= 0 {
		limit = global.DefaultLimit
	}

	list, _, err := s.loadList(source, project, playbook, listName)
	if err != nil {
		return nil, err
	}

	// Filter items first (if complete filter is specified for project lists)
	var filteredItems []global.ListItem
	for _, item := range list.Items {
		// Complete filter (only for project lists)
		if completeFilter != "" && (source == SourceProject || source == "") {
			if completeFilter == "true" && !item.Complete {
				continue
			}
			if completeFilter == "false" && item.Complete {
				continue
			}
		}
		filteredItems = append(filteredItems, item)
	}

	// Apply pagination to filtered items
	total := len(filteredItems)
	if offset >= total {
		return &global.ListGetSummaryResponse{
			Name:          list.Name,
			Description:   list.Description,
			ItemCount:     len(list.Items), // Total items in list (unfiltered)
			Items:         []global.ListItemSummary{},
			ReturnedCount: 0,
			Offset:        offset,
		}, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	// Build summaries with truncated content
	var summaries []global.ListItemSummary
	for _, item := range filteredItems[offset:end] {
		content := item.Content
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		summaries = append(summaries, global.ListItemSummary{
			ID:       item.ID,
			Title:    item.Title,
			Content:  content,
			Complete: item.Complete,
		})
	}

	s.logger.Debugf("Retrieved list summary: %s (%d items, %d filtered)", listName, len(list.Items), total)
	return &global.ListGetSummaryResponse{
		Name:          list.Name,
		Description:   list.Description,
		ItemCount:     len(list.Items), // Total items in list (unfiltered)
		Items:         summaries,
		ReturnedCount: len(summaries),
		Offset:        offset,
	}, nil
}

// Create creates a new list.
// The listName parameter should be the list name without .json extension.
func (s *Service) Create(source, project, playbook, listName, name, description string, items []global.ListItem) error {
	if !isWritable(source) {
		return fmt.Errorf("cannot create list in read-only source: %s", source)
	}

	filename, err := normalizeListName(listName)
	if err != nil {
		return err
	}

	if name == "" {
		return fmt.Errorf("list name is required")
	}

	listDir, err := s.resolveListDir(source, project, playbook)
	if err != nil {
		return err
	}

	filePath := filepath.Join(listDir, filename)

	mutex := s.getPathMutex(filePath)
	mutex.Lock()
	defer mutex.Unlock()

	// Check if already exists
	if global.FileExists(filePath) {
		return fmt.Errorf("list already exists: %s", filename)
	}

	// Validate initial items if provided
	if items == nil {
		items = []global.ListItem{}
	}

	// Check for duplicate IDs in initial items
	idSet := make(map[string]bool)
	for i, item := range items {
		if err := validateItem(&items[i]); err != nil {
			return fmt.Errorf("invalid item at index %d: %w", i, err)
		}
		if idSet[item.ID] {
			return fmt.Errorf("duplicate item id: %s", item.ID)
		}
		idSet[item.ID] = true
	}

	now := time.Now()
	list := &global.List{
		Version:     global.ListSchemaVersion,
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
		Items:       items,
	}

	if err := s.saveList(filePath, list); err != nil {
		return err
	}

	s.logger.Infof("Created list: %s with %d items", filename, len(items))
	return nil
}

// Delete deletes a list.
// The listName parameter should be the list name without .json extension.
func (s *Service) Delete(source, project, playbook, listName string) error {
	if !isWritable(source) {
		return fmt.Errorf("cannot delete list in read-only source: %s", source)
	}

	filename, err := normalizeListName(listName)
	if err != nil {
		return err
	}

	listDir, err := s.resolveListDir(source, project, playbook)
	if err != nil {
		return err
	}

	filePath := filepath.Join(listDir, filename)

	mutex := s.getPathMutex(filePath)
	mutex.Lock()
	defer mutex.Unlock()

	if !global.FileExists(filePath) {
		return fmt.Errorf("list not found: %s", filename)
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete list: %w", err)
	}

	s.logger.Infof("Deleted list: %s", filename)
	return nil
}

// Rename renames a list.
// The listName and newListName parameters should be list names without .json extension.
func (s *Service) Rename(source, project, playbook, listName, newListName string) error {
	if !isWritable(source) {
		return fmt.Errorf("cannot rename list in read-only source: %s", source)
	}

	filename, err := normalizeListName(listName)
	if err != nil {
		return err
	}
	newFilename, err := normalizeListName(newListName)
	if err != nil {
		return err
	}

	listDir, err := s.resolveListDir(source, project, playbook)
	if err != nil {
		return err
	}

	oldPath := filepath.Join(listDir, filename)
	newPath := filepath.Join(listDir, newFilename)

	mutex1 := s.getPathMutex(oldPath)
	mutex2 := s.getPathMutex(newPath)
	mutex1.Lock()
	defer mutex1.Unlock()
	mutex2.Lock()
	defer mutex2.Unlock()

	if !global.FileExists(oldPath) {
		return fmt.Errorf("list not found: %s", filename)
	}

	if global.FileExists(newPath) {
		return fmt.Errorf("list already exists: %s", newFilename)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("failed to rename list: %w", err)
	}

	s.logger.Infof("Renamed list: %s -> %s", filename, newFilename)
	return nil
}

// Copy copies a list from one location to another.
// This supports copying between projects, playbooks, and reference (read-only source).
// The from/to list names should not include .json extension.
// If sample > 0, randomly selects that many items from the source list.
func (s *Service) Copy(
	fromSource, fromProject, fromPlaybook, fromListName string,
	toSource, toProject, toPlaybook, toListName string,
	sample int,
) error {
	// Validate destination is writable
	if !isWritable(toSource) {
		return fmt.Errorf("cannot copy list to read-only destination: %s", toSource)
	}

	// Normalize list names
	fromFilename, err := normalizeListName(fromListName)
	if err != nil {
		return fmt.Errorf("invalid source list name: %w", err)
	}
	toFilename, err := normalizeListName(toListName)
	if err != nil {
		return fmt.Errorf("invalid destination list name: %w", err)
	}

	// Load source list
	sourceList, _, err := s.loadList(fromSource, fromProject, fromPlaybook, fromListName)
	if err != nil {
		return fmt.Errorf("failed to load source list: %w", err)
	}

	// Resolve destination directory
	destListDir, err := s.resolveListDir(toSource, toProject, toPlaybook)
	if err != nil {
		return fmt.Errorf("failed to resolve destination: %w", err)
	}

	destPath := filepath.Join(destListDir, toFilename)

	mutex := s.getPathMutex(destPath)
	mutex.Lock()
	defer mutex.Unlock()

	// Check if destination already exists
	if global.FileExists(destPath) {
		return fmt.Errorf("destination list already exists: %s", toListName)
	}

	// Determine which items to copy (all or sampled)
	itemsToCopy := sourceList.Items
	if sample > 0 && sample < len(sourceList.Items) {
		itemsToCopy = s.randomSample(sourceList.Items, sample)
		s.logger.Infof("Sampling %d of %d items from list '%s'", sample, len(sourceList.Items), fromListName)
	}

	// Create a copy of the list with updated timestamps
	now := time.Now()
	copiedList := &global.List{
		Version:     sourceList.Version,
		Name:        sourceList.Name,
		Description: sourceList.Description,
		Templates:   sourceList.Templates,
		CreatedAt:   now,
		UpdatedAt:   now,
		Items:       make([]global.ListItem, len(itemsToCopy)),
	}
	copy(copiedList.Items, itemsToCopy)

	// Save to destination
	if err := s.saveList(destPath, copiedList); err != nil {
		return fmt.Errorf("failed to save copied list: %w", err)
	}

	s.logger.Infof("Copied list from %s/%s to %s/%s", fromSource, fromFilename, toSource, toFilename)
	return nil
}

// AddItem adds a new item to a list.
// The listName parameter should be the list name without .json extension.
// If item.ID is empty, an ID will be auto-generated (item-001, item-002, etc.).
// For playbook lists, complete must be false - playbook items cannot be marked complete.
// Returns the assigned item ID (useful when auto-generated).
func (s *Service) AddItem(source, project, playbook, listName string, item *global.ListItem) (string, error) {
	if !isWritable(source) {
		return "", fmt.Errorf("cannot modify list in read-only source: %s", source)
	}

	// Enforce playbook restriction: items cannot be marked complete
	if source == SourcePlaybook && item.Complete {
		return "", fmt.Errorf("playbook list items cannot be marked complete - copy the list to a project first")
	}

	if err := validateItem(item); err != nil {
		return "", err
	}

	list, filePath, err := s.loadList(source, project, playbook, listName)
	if err != nil {
		return "", err
	}

	mutex := s.getPathMutex(filePath)
	mutex.Lock()
	defer mutex.Unlock()

	// Always auto-generate ID - any provided ID is ignored
	item.ID = generateItemID(list.Items)

	list.Items = append(list.Items, *item)

	if err := s.saveList(filePath, list); err != nil {
		return "", err
	}

	s.logger.Infof("Added item '%s' to list: %s", item.ID, listName)
	return item.ID, nil
}

// UpdateItem updates an existing item in a list.
// The listName parameter should be the list name without .json extension.
// For playbook lists, complete cannot be set to true - playbook items cannot be marked complete.
func (s *Service) UpdateItem(source, project, playbook, listName, itemID string, title, content, sourceDoc, section *string, tags []string, clearTags bool, complete *bool) error {
	if !isWritable(source) {
		return fmt.Errorf("cannot modify list in read-only source: %s", source)
	}

	// Enforce playbook restriction: items cannot be marked complete
	if source == SourcePlaybook && complete != nil && *complete {
		return fmt.Errorf("playbook list items cannot be marked complete - copy the list to a project first")
	}

	list, filePath, err := s.loadList(source, project, playbook, listName)
	if err != nil {
		return err
	}

	mutex := s.getPathMutex(filePath)
	mutex.Lock()
	defer mutex.Unlock()

	// Find item
	found := false
	for i := range list.Items {
		if list.Items[i].ID == itemID {
			found = true
			// Update only specified fields
			if title != nil {
				if *title == "" {
					return fmt.Errorf("title cannot be empty")
				}
				list.Items[i].Title = *title
			}
			if content != nil {
				if *content == "" {
					return fmt.Errorf("content cannot be empty")
				}
				list.Items[i].Content = *content
			}
			if sourceDoc != nil {
				list.Items[i].SourceDoc = *sourceDoc
			}
			if section != nil {
				list.Items[i].Section = *section
			}
			if clearTags {
				list.Items[i].Tags = nil
			} else if tags != nil {
				list.Items[i].Tags = tags
			}
			if complete != nil {
				list.Items[i].Complete = *complete
			}
			break
		}
	}

	if !found {
		return fmt.Errorf("item not found: %s", itemID)
	}

	if err := s.saveList(filePath, list); err != nil {
		return err
	}

	s.logger.Infof("Updated item '%s' in list: %s", itemID, listName)
	return nil
}

// RemoveItem removes an item from a list.
// The listName parameter should be the list name without .json extension.
func (s *Service) RemoveItem(source, project, playbook, listName, itemID string) error {
	if !isWritable(source) {
		return fmt.Errorf("cannot modify list in read-only source: %s", source)
	}

	list, filePath, err := s.loadList(source, project, playbook, listName)
	if err != nil {
		return err
	}

	mutex := s.getPathMutex(filePath)
	mutex.Lock()
	defer mutex.Unlock()

	// Find and remove item
	found := false
	for i := range list.Items {
		if list.Items[i].ID == itemID {
			found = true
			list.Items = append(list.Items[:i], list.Items[i+1:]...)
			break
		}
	}

	if !found {
		return fmt.Errorf("item not found: %s", itemID)
	}

	if err := s.saveList(filePath, list); err != nil {
		return err
	}

	s.logger.Infof("Removed item '%s' from list: %s", itemID, listName)
	return nil
}

// RenameItem renames an item's ID in a list.
// The listName parameter should be the list name without .json extension.
func (s *Service) RenameItem(source, project, playbook, listName, itemID, newItemID string) error {
	if !isWritable(source) {
		return fmt.Errorf("cannot modify list in read-only source: %s", source)
	}

	if newItemID == "" {
		return fmt.Errorf("new item id cannot be empty")
	}

	list, filePath, err := s.loadList(source, project, playbook, listName)
	if err != nil {
		return err
	}

	mutex := s.getPathMutex(filePath)
	mutex.Lock()
	defer mutex.Unlock()

	// Check new ID doesn't exist
	for _, item := range list.Items {
		if item.ID == newItemID {
			return fmt.Errorf("item with id '%s' already exists", newItemID)
		}
	}

	// Find and rename item
	found := false
	for i := range list.Items {
		if list.Items[i].ID == itemID {
			found = true
			list.Items[i].ID = newItemID
			break
		}
	}

	if !found {
		return fmt.Errorf("item not found: %s", itemID)
	}

	if err := s.saveList(filePath, list); err != nil {
		return err
	}

	s.logger.Infof("Renamed item '%s' to '%s' in list: %s", itemID, newItemID, listName)
	return nil
}

// GetItem returns a single item from a list.
// The listName parameter should be the list name without .json extension.
func (s *Service) GetItem(source, project, playbook, listName, itemID string) (*global.ListItem, error) {
	list, _, err := s.loadList(source, project, playbook, listName)
	if err != nil {
		return nil, err
	}

	for _, item := range list.Items {
		if item.ID == itemID {
			s.logger.Debugf("Retrieved item '%s' from list: %s", itemID, listName)
			return &item, nil
		}
	}

	return nil, fmt.Errorf("item not found: %s", itemID)
}

// SearchItems searches for items in a list.
// The listName parameter should be the list name without .json extension.
// The completeFilter parameter is only used for project lists: "true", "false", or "" (no filter).
func (s *Service) SearchItems(source, project, playbook, listName, query, sourceDoc, section string, tags []string, completeFilter string, offset, limit int) (*global.ListItemSearchResponse, error) {
	if limit <= 0 {
		limit = global.DefaultLimit
	}

	list, _, err := s.loadList(source, project, playbook, listName)
	if err != nil {
		return nil, err
	}

	// Filter items
	var matches []global.ListItem
	queryLower := strings.ToLower(query)

	for _, item := range list.Items {
		// Query filter (case-insensitive substring on id OR content)
		if query != "" {
			idMatch := strings.Contains(strings.ToLower(item.ID), queryLower)
			contentMatch := strings.Contains(strings.ToLower(item.Content), queryLower)
			if !idMatch && !contentMatch {
				continue
			}
		}

		// Source doc filter (exact match)
		if sourceDoc != "" && item.SourceDoc != sourceDoc {
			continue
		}

		// Section filter (exact match)
		if section != "" && item.Section != section {
			continue
		}

		// Tags filter (AND logic - must have all)
		if len(tags) > 0 {
			hasAllTags := true
			for _, reqTag := range tags {
				found := false
				for _, itemTag := range item.Tags {
					if itemTag == reqTag {
						found = true
						break
					}
				}
				if !found {
					hasAllTags = false
					break
				}
			}
			if !hasAllTags {
				continue
			}
		}

		// Complete filter (only for project lists)
		if completeFilter != "" && (source == SourceProject || source == "") {
			if completeFilter == "true" && !item.Complete {
				continue
			}
			if completeFilter == "false" && item.Complete {
				continue
			}
		}

		matches = append(matches, item)
	}

	// Apply pagination
	total := len(matches)
	if offset >= total {
		return &global.ListItemSearchResponse{
			Items:         []global.ListItem{},
			TotalCount:    total,
			ReturnedCount: 0,
			Offset:        offset,
		}, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	result := matches[offset:end]

	s.logger.Debugf("Search in list %s: found %d matches (returned %d)", listName, total, len(result))
	return &global.ListItemSearchResponse{
		Items:         result,
		TotalCount:    total,
		ReturnedCount: len(result),
		Offset:        offset,
	}, nil
}

// TaskCreator interface for creating tasks and managing tasksets (to avoid circular dependency)
type TaskCreator interface {
	CreateTask(project, path, title, taskType string, work *global.WorkExecution, qa *global.QAExecution) (*global.Task, error)
	GetTaskSet(project, path string) (*global.TaskSet, error)
	CreateTaskSet(project, path, title, description string, templates *global.DefaultTemplates, parallel bool, limits global.Limits) (*global.TaskSet, error)
}

// CreateTasks creates tasks from list items.
// The listName parameter should be the list name without .json extension.
// The priority parameter is reserved for future use.
// The qaTemplate parameter, if non-nil, enables QA for all created tasks.
// The sample parameter, if > 0, randomly selects that many items from the list.
// The parallel parameter enables parallel task execution in the created taskset.
func (s *Service) CreateTasks(
	taskCreator TaskCreator,
	listSource, project, playbook, listName string,
	targetProject, path string,
	titleTemplate, taskType string, priority int,
	llmModelID, instructionsFile, instructionsFileSource, instructionsText, basePrompt string,
	qaTemplate *global.QAExecution,
	sample int,
	parallel bool,
) (*global.ListCreateTasksResponse, error) {
	// Load the list
	list, _, err := s.loadList(listSource, project, playbook, listName)
	if err != nil {
		return nil, err
	}

	if len(list.Items) == 0 {
		return &global.ListCreateTasksResponse{
			TasksCreated: 0,
			ListName:     list.Name,
			ItemCount:    0,
			TaskIDs:      []int{},
		}, nil
	}

	// Ensure taskset exists, creating it with list templates if needed
	_, err = taskCreator.GetTaskSet(targetProject, path)
	if err != nil {
		// Taskset doesn't exist, create it with list templates
		tasksetTitle := list.Name
		if tasksetTitle == "" {
			tasksetTitle = listName
		}
		_, err = taskCreator.CreateTaskSet(
			targetProject,
			path,
			tasksetTitle,
			"", // description - empty for now
			list.Templates,
			parallel,
			global.Limits{}, // use defaults
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create task set: %w", err)
		}
		s.logger.Infof("Created task set '%s' with list templates", path)
	}

	// If sample is specified, randomly select that many items
	items := list.Items
	if sample > 0 && sample < len(list.Items) {
		items = s.randomSample(list.Items, sample)
		s.logger.Infof("Sampling %d of %d items from list '%s'", sample, len(list.Items), listName)
	}

	// Default title template
	if titleTemplate == "" {
		titleTemplate = "{{title}}"
	}

	// Note: priority is reserved for future use when task prioritization is implemented
	_ = priority

	var taskIDs []int
	for _, item := range items {
		// Build task title from template (supports {{title}} and {{id}} placeholders)
		title := titleTemplate
		title = strings.ReplaceAll(title, "{{title}}", item.Title)
		title = strings.ReplaceAll(title, "{{id}}", item.ID)

		// Build item context to append to prompt
		var itemContext strings.Builder
		itemContext.WriteString("\n=== LIST ITEM ===\n")
		itemContext.WriteString(fmt.Sprintf("ID: %s\n", item.ID))
		itemContext.WriteString(fmt.Sprintf("Title: %s\n", item.Title))
		itemContext.WriteString(fmt.Sprintf("Content: %s\n", item.Content))
		if item.SourceDoc != "" {
			itemContext.WriteString(fmt.Sprintf("Source: %s\n", item.SourceDoc))
		}
		if item.Section != "" {
			itemContext.WriteString(fmt.Sprintf("Section: %s\n", item.Section))
		}
		if len(item.Tags) > 0 {
			itemContext.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(item.Tags, ", ")))
		}

		// Combine base prompt with item context
		fullPrompt := basePrompt + itemContext.String()

		// Create work execution object
		work := &global.WorkExecution{
			LLMModelID:             llmModelID,
			InstructionsFile:       instructionsFile,
			InstructionsFileSource: instructionsFileSource,
			InstructionsText:       instructionsText,
			Prompt:                 fullPrompt,
			Status:                 global.ExecutionStatusWaiting,
		}

		// Create QA execution object from template or auto-enable from list templates
		var qa *global.QAExecution
		if qaTemplate != nil && qaTemplate.Enabled {
			// Explicit QA config provided
			qa = &global.QAExecution{
				Enabled:                true,
				InstructionsFile:       qaTemplate.InstructionsFile,
				InstructionsFileSource: qaTemplate.InstructionsFileSource,
				InstructionsText:       qaTemplate.InstructionsText,
				Prompt:                 qaTemplate.Prompt,
				LLMModelID:             qaTemplate.LLMModelID,
			}
		} else if list.Templates != nil && (list.Templates.QAResponseTemplate != "" || list.Templates.QAReportTemplate != "") {
			// List has QA templates - auto-enable QA
			s.logger.Infof("Auto-enabling QA for task based on list templates")
			qa = &global.QAExecution{
				Enabled: true,
			}
		} else {
			qa = &global.QAExecution{
				Enabled: false,
			}
		}

		// Create the task
		task, err := taskCreator.CreateTask(
			targetProject,
			path,
			title,
			taskType,
			work,
			qa,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create task for item '%s': %w", item.ID, err)
		}

		taskIDs = append(taskIDs, task.ID)
	}

	s.logger.Infof("Created %d tasks from list '%s'", len(taskIDs), listName)
	return &global.ListCreateTasksResponse{
		TasksCreated: len(taskIDs),
		ListName:     list.Name,
		ItemCount:    len(list.Items),
		TaskIDs:      taskIDs,
	}, nil
}

// randomSample returns a random sample of n items from the given slice.
// Uses Fisher-Yates shuffle on a copy to avoid modifying the original.
func (s *Service) randomSample(items []global.ListItem, n int) []global.ListItem {
	if n >= len(items) {
		return items
	}

	// Create a copy to shuffle
	shuffled := make([]global.ListItem, len(items))
	copy(shuffled, items)

	// Fisher-Yates shuffle
	for i := len(shuffled) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	return shuffled[:n]
}
