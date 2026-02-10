/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package library

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/global"
)

// ListResult represents the result of a list operation
type ListResult struct {
	Items []*Item `json:"items"`
	Total int     `json:"total"`
}

// ListItems lists items in a category and optional path prefix
func (s *Service) ListItems(category, prefix string, limit, offset int, includeSummary bool) (*ListResult, error) {
	// Set defaults
	if limit <= 0 {
		limit = global.DefaultLimit
	}

	var items []*Item

	if category != "" {
		// List specific category
		cat, err := s.validateCategory(category)
		if err != nil {
			return nil, err
		}

		categoryItems, err := s.listCategoryItems(cat, prefix, includeSummary)
		if err != nil {
			return nil, err
		}
		items = append(items, categoryItems...)
	} else {
		// List all categories
		for _, cat := range s.categories {
			categoryItems, err := s.listCategoryItems(cat, prefix, includeSummary)
			if err != nil {
				s.logger.Warnf("Failed to list items in category %s: %v", cat.Name, err)
				continue
			}
			items = append(items, categoryItems...)
		}
	}

	// Sort by modified time (newest first)
	sort.Slice(items, func(i, j int) bool {
		return items[i].ModifiedAt.After(items[j].ModifiedAt)
	})

	// Apply pagination
	total := len(items)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	if start >= total {
		items = []*Item{}
	} else {
		items = items[start:end]
	}

	s.logger.Debugf("Listed %d items (total: %d, category: %s, prefix: %s)", len(items), total, category, prefix)

	return &ListResult{
		Items: items,
		Total: total,
	}, nil
}

// listCategoryItems lists all items in a specific category
func (s *Service) listCategoryItems(cat *config.Category, prefix string, includeSummary bool) ([]*Item, error) {
	var items []*Item

	err := filepath.WalkDir(cat.Directory, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip metadata files
		if strings.HasSuffix(path, global.MetaSuffix) {
			return nil
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get relative path from category directory
		relPath, err := filepath.Rel(cat.Directory, path)
		if err != nil {
			s.logger.Warnf("Failed to get relative path for %s: %v", path, err)
			return nil // Continue walking
		}

		// Convert to forward slashes for consistent key format
		relPath = strings.ReplaceAll(relPath, string(filepath.Separator), "/")

		// Apply prefix filter
		if prefix != "" && !strings.HasPrefix(relPath, prefix) {
			return nil
		}

		// Create item without content
		item, err := s.createItemFromPath(path, cat.Name, relPath, false)
		if err != nil {
			s.logger.Warnf("Failed to create item for %s: %v", path, err)
			return nil // Continue walking
		}

		// Remove summary if not requested
		if !includeSummary {
			item.Summary = ""
		}

		items = append(items, item)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", cat.Directory, err)
	}

	return items, nil
}

// SearchItems searches for items by substring in keys and optionally contents
func (s *Service) SearchItems(category, query string, searchInContent bool, limit, offset int) (*ListResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	// Set defaults
	if limit <= 0 {
		limit = global.DefaultLimit
	}

	var items []*Item

	if category != "" {
		// Search specific category
		cat, err := s.validateCategory(category)
		if err != nil {
			return nil, err
		}

		categoryItems, err := s.searchCategoryItems(cat, query, searchInContent)
		if err != nil {
			return nil, err
		}
		items = append(items, categoryItems...)
	} else {
		// Search all categories
		for _, cat := range s.categories {
			categoryItems, err := s.searchCategoryItems(cat, query, searchInContent)
			if err != nil {
				s.logger.Warnf("Failed to search items in category %s: %v", cat.Name, err)
				continue
			}
			items = append(items, categoryItems...)
		}
	}

	// Sort by relevance (key matches first, then modified time)
	sort.Slice(items, func(i, j int) bool {
		iKeyMatch := strings.Contains(strings.ToLower(items[i].Key), strings.ToLower(query))
		jKeyMatch := strings.Contains(strings.ToLower(items[j].Key), strings.ToLower(query))

		if iKeyMatch && !jKeyMatch {
			return true
		}
		if !iKeyMatch && jKeyMatch {
			return false
		}

		// Both or neither match key, sort by modified time
		return items[i].ModifiedAt.After(items[j].ModifiedAt)
	})

	// Apply pagination
	total := len(items)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	if start >= total {
		items = []*Item{}
	} else {
		items = items[start:end]
	}

	s.logger.Debugf("Found %d items matching '%s' (total: %d, category: %s)", len(items), query, total, category)

	return &ListResult{
		Items: items,
		Total: total,
	}, nil
}

// searchCategoryItems searches items in a specific category
func (s *Service) searchCategoryItems(cat *config.Category, query string, searchInContent bool) ([]*Item, error) {
	var items []*Item
	queryLower := strings.ToLower(query)

	err := filepath.WalkDir(cat.Directory, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip metadata files
		if strings.HasSuffix(path, global.MetaSuffix) {
			return nil
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get relative path from category directory
		relPath, err := filepath.Rel(cat.Directory, path)
		if err != nil {
			s.logger.Warnf("Failed to get relative path for %s: %v", path, err)
			return nil
		}

		// Convert to forward slashes for consistent key format
		relPath = strings.ReplaceAll(relPath, string(filepath.Separator), "/")
		key := cat.Name + "/" + relPath

		// Check if key matches
		keyMatches := strings.Contains(strings.ToLower(key), queryLower)

		// Check content if requested and key doesn't match
		contentMatches := false
		if searchInContent && !keyMatches {
			// Only read file for content search if it's valid UTF-8
			if err := isValidUTF8File(path); err == nil {
				if content, err := os.ReadFile(path); err == nil {
					contentMatches = strings.Contains(strings.ToLower(string(content)), queryLower)
				}
			}
		}

		if keyMatches || contentMatches {
			// Create item without content
			item, err := s.createItemFromPath(path, cat.Name, relPath, false)
			if err != nil {
				s.logger.Warnf("Failed to create item for %s: %v", path, err)
				return nil
			}

			// Set summary to indicate match type
			if contentMatches {
				item.Summary = fmt.Sprintf("Contains '%s'", query)
			}

			items = append(items, item)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", cat.Directory, err)
	}

	return items, nil
}

// RenameItem renames or moves an item from one key to another
func (s *Service) RenameItem(fromKey, toKey string, overwrite bool) (bool, bool, error) {
	fromPath, fromCat, err := s.resolveFilePath(fromKey)
	if err != nil {
		return false, false, err
	}

	toPath, toCat, err := s.resolveFilePath(toKey)
	if err != nil {
		return false, false, err
	}

	// Check read-only restrictions
	if fromCat.ReadOnly {
		return false, false, fmt.Errorf("source category %s is read-only", fromCat.Name)
	}

	// Lock both paths (in consistent order to prevent deadlock)
	paths := []string{fromPath, toPath}
	sort.Strings(paths)

	mutex1 := s.getPathMutex(paths[0])
	mutex2 := s.getPathMutex(paths[1])

	mutex1.Lock()
	defer mutex1.Unlock()

	if paths[0] != paths[1] {
		mutex2.Lock()
		defer mutex2.Unlock()
	}

	// Check if source exists
	if _, err := os.Stat(fromPath); os.IsNotExist(err) {
		return false, false, fmt.Errorf("source item not found: %s", fromKey)
	}

	// Check if destination exists
	_, destErr := os.Stat(toPath)
	destExists := destErr == nil

	if destExists {
		if toCat.ReadOnly {
			return false, false, fmt.Errorf("destination category %s is read-only", toCat.Name)
		}
		if !overwrite {
			return false, false, fmt.Errorf("destination exists and overwrite is false")
		}
	} else if toCat.ReadOnly {
		return false, false, fmt.Errorf("destination category %s is read-only", toCat.Name)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(toPath), 0755); err != nil {
		return false, false, fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Move the file
	if err := os.Rename(fromPath, toPath); err != nil {
		return false, false, fmt.Errorf("failed to move file: %w", err)
	}

	// Move metadata file if it exists
	fromMetaPath := fromPath + global.MetaSuffix
	toMetaPath := toPath + global.MetaSuffix

	if _, err := os.Stat(fromMetaPath); err == nil {
		if err := os.Rename(fromMetaPath, toMetaPath); err != nil {
			s.logger.Warnf("Failed to move metadata file from %s to %s: %v", fromMetaPath, toMetaPath, err)
		}
	}

	overwritten := destExists

	s.logger.Debugf("Renamed item: %s -> %s (overwritten=%t)", fromKey, toKey, overwritten)
	return true, overwritten, nil
}

// DeleteItem deletes an item by key
func (s *Service) DeleteItem(key string) (bool, error) {
	filePath, cat, err := s.resolveFilePath(key)
	if err != nil {
		return false, err
	}

	// Check read-only category
	if cat.ReadOnly {
		return false, fmt.Errorf("category %s is read-only", cat.Name)
	}

	mutex := s.getPathMutex(filePath)
	mutex.Lock()
	defer mutex.Unlock()

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false, fmt.Errorf("item not found: %s", key)
	}

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		return false, fmt.Errorf("failed to delete file: %w", err)
	}

	// Delete metadata file if it exists
	metaPath := filePath + global.MetaSuffix
	if _, err := os.Stat(metaPath); err == nil {
		if err := os.Remove(metaPath); err != nil {
			s.logger.Warnf("Failed to delete metadata file %s: %v", metaPath, err)
		}
	}

	s.logger.Debugf("Deleted item: %s", key)
	return true, nil
}

// CategoryInfo represents category information
type CategoryInfo struct {
	Name      string `json:"name"`
	Directory string `json:"directory"`
	ReadOnly  bool   `json:"read_only"`
}

// ListCategories returns the list of configured categories
func (s *Service) ListCategories() ([]*CategoryInfo, error) {
	categories := make([]*CategoryInfo, 0, len(s.categories))

	for _, cat := range s.categories {
		categories = append(categories, &CategoryInfo{
			Name:      cat.Name,
			Directory: cat.Directory,
			ReadOnly:  cat.ReadOnly,
		})
	}

	// Sort by name for consistent output
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})

	return categories, nil
}
