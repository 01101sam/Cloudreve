package user

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"

	"github.com/cloudreve/Cloudreve/v4/application/dependency"
	"github.com/cloudreve/Cloudreve/v4/inventory"
	"github.com/cloudreve/Cloudreve/v4/pkg/serializer"
	"github.com/gin-gonic/gin"
)

// ViewPreferenceData represents view preferences for a folder
type ViewPreferenceData struct {
	Layout        string `json:"layout,omitempty"`
	ShowThumb     bool   `json:"show_thumb"`
	SortBy        string `json:"sort_by,omitempty"`
	SortDirection string `json:"sort_direction,omitempty"`
	PageSize      int    `json:"page_size,omitempty"`
	GalleryWidth  int    `json:"gallery_width,omitempty"`
	ListColumns   string `json:"list_columns,omitempty"`
}

// ViewPreferenceResponse represents the API response for view preferences
type ViewPreferenceResponse struct {
	Layout        string `json:"layout,omitempty"`
	ShowThumb     bool   `json:"show_thumb"`
	SortBy        string `json:"sort_by,omitempty"`
	SortDirection string `json:"sort_direction,omitempty"`
	PageSize      int    `json:"page_size,omitempty"`
	GalleryWidth  int    `json:"gallery_width,omitempty"`
	ListColumns   string `json:"list_columns,omitempty"`
}

// makeViewPrefKey creates a key for storing view preferences
func makeViewPrefKey(userID int, folderPath string) string {
	return fmt.Sprintf("view_pref_%d_%s", userID, folderPath)
}

// GetFolderViewPreference retrieves view preferences for a specific folder
func GetFolderViewPreference(c *gin.Context, folderPath string) (*ViewPreferenceData, error) {
	user := inventory.UserFromContext(c)
	dep := dependency.FromContext(c)

	// Check if user has sync enabled
	if !user.Settings.SyncViewPreferences {
		// Return default preferences when sync is disabled
		return getDefaultViewPreference(), nil
	}

	// Normalize folder path
	folderPath = path.Clean(folderPath)
	if folderPath == "." {
		folderPath = "/"
	}

	// Try to get preferences from KV store
	kv := dep.KV()
	key := makeViewPrefKey(user.ID, folderPath)

	data, ok := kv.Get(key)
	if !ok {
		// If not found for this path, try parent paths
		if folderPath != "/" {
			parentPath := path.Dir(folderPath)
			return GetFolderViewPreference(c, parentPath)
		}
		// Return default if no preferences found
		return getDefaultViewPreference(), nil
	}

	// Parse the stored JSON
	var prefs ViewPreferenceData
	if jsonData, ok := data.(string); ok {
		if err := json.Unmarshal([]byte(jsonData), &prefs); err != nil {
			return getDefaultViewPreference(), nil
		}
		return &prefs, nil
	}

	return getDefaultViewPreference(), nil
}

// SetFolderViewPreference saves or updates view preferences for a folder
func SetFolderViewPreference(c *gin.Context, folderPath string, prefs *ViewPreferenceData) error {
	user := inventory.UserFromContext(c)
	dep := dependency.FromContext(c)

	// Check if user has sync enabled
	if !user.Settings.SyncViewPreferences {
		// Silently do nothing when sync is disabled
		return nil
	}

	// Normalize folder path
	folderPath = path.Clean(folderPath)
	if folderPath == "." {
		folderPath = "/"
	}

	// Check if preferences are same as parent
	if folderPath != "/" {
		parentPath := path.Dir(folderPath)
		parentPrefs, _ := GetFolderViewPreference(c, parentPath)
		if isPreferenceEqual(prefs, parentPrefs) {
			// Remove redundant preference
			kv := dep.KV()
			key := makeViewPrefKey(user.ID, folderPath)
			kv.Delete(key)
			return nil
		}
	}

	// Store preferences in KV store
	kv := dep.KV()
	key := makeViewPrefKey(user.ID, folderPath)

	jsonData, err := json.Marshal(prefs)
	if err != nil {
		return serializer.NewError(serializer.CodeInternalSetting, "Failed to serialize preferences", err)
	}

	// Store with no expiration (0 means permanent)
	if err := kv.Set(key, string(jsonData), 0); err != nil {
		return serializer.NewError(serializer.CodeInternalSetting, "Failed to store preferences", err)
	}

	return nil
}

// DeleteFolderViewPreferences deletes all view preferences for folders with the given paths
func DeleteFolderViewPreferences(ctx context.Context, userID int, folderPaths []string) error {
	if len(folderPaths) == 0 {
		return nil
	}

	dep := dependency.FromContext(ctx)
	kv := dep.KV()

	// Normalize folder paths and create keys
	keys := make([]string, 0, len(folderPaths))
	for _, folderPath := range folderPaths {
		folderPath = path.Clean(folderPath)
		if folderPath == "." {
			folderPath = "/"
		}
		keys = append(keys, makeViewPrefKey(userID, folderPath))
	}

	// Delete all keys
	for _, key := range keys {
		kv.Delete(key)
	}
	return nil
}

// getDefaultViewPreference returns the default view preferences
func getDefaultViewPreference() *ViewPreferenceData {
	return &ViewPreferenceData{
		Layout:        "grid",
		ShowThumb:     true,
		SortBy:        "created_at",
		SortDirection: "asc",
		PageSize:      100,
		GalleryWidth:  220,
		ListColumns:   "",
	}
}

// isPreferenceEqual checks if two view preferences are equal
func isPreferenceEqual(a, b *ViewPreferenceData) bool {
	if a.Layout != b.Layout || a.ShowThumb != b.ShowThumb ||
		a.SortBy != b.SortBy || a.SortDirection != b.SortDirection ||
		a.PageSize != b.PageSize || a.GalleryWidth != b.GalleryWidth ||
		a.ListColumns != b.ListColumns {
		return false
	}

	return true
}

// ViewPreferenceService handles view preference API requests
type (
	// GetViewPreferenceService Service to get view preferences for a folder
	GetViewPreferenceService struct {
		Path string `json:"path" binding:"required"`
	}
	GetViewPreferenceParamCtx struct{}

	// SetViewPreferenceService Service to set view preferences for a folder
	SetViewPreferenceService struct {
		Path          string  `json:"path" binding:"required"`
		Layout        *string `json:"layout" binding:"omitempty,oneof=grid list gallery"`
		ShowThumb     *bool   `json:"show_thumb" binding:"omitempty"`
		SortBy        *string `json:"sort_by" binding:"omitempty"`
		SortDirection *string `json:"sort_direction" binding:"omitempty,oneof=asc desc"`
		PageSize      *int    `json:"page_size" binding:"omitempty,min=10,max=2000"`
		GalleryWidth  *int    `json:"gallery_width" binding:"omitempty,min=50,max=500"`
		ListColumns   *string `json:"list_columns" binding:"omitempty"`
	}
	SetViewPreferenceParamCtx struct{}
)

// GetViewPreference retrieves view preferences for a folder with inheritance
func (s *GetViewPreferenceService) Get(c *gin.Context) (*ViewPreferenceResponse, error) {
	u := inventory.UserFromContext(c)

	// If sync is disabled, return empty response
	if !u.Settings.SyncViewPreferences {
		return &ViewPreferenceResponse{}, nil
	}

	// Clean and validate the path
	path := filepath.Clean(s.Path)
	if path == "." {
		path = "/"
	}

	// Get view preferences
	prefs, err := GetFolderViewPreference(c, path)
	if err != nil {
		return nil, err
	}

	// Create response
	response := &ViewPreferenceResponse{
		Layout:        prefs.Layout,
		ShowThumb:     prefs.ShowThumb,
		SortBy:        prefs.SortBy,
		SortDirection: prefs.SortDirection,
		PageSize:      prefs.PageSize,
		GalleryWidth:  prefs.GalleryWidth,
		ListColumns:   prefs.ListColumns,
	}

	return response, nil
}

// Set updates view preferences for a folder
func (s *SetViewPreferenceService) Set(c *gin.Context) error {
	u := inventory.UserFromContext(c)

	// If sync is disabled, do nothing
	if !u.Settings.SyncViewPreferences {
		return nil
	}

	// Clean and validate the path
	path := filepath.Clean(s.Path)
	if path == "." {
		path = "/"
	}

	// Build preference data from request
	data := ViewPreferenceData{}

	if s.Layout != nil {
		data.Layout = *s.Layout
	}
	if s.ShowThumb != nil {
		data.ShowThumb = *s.ShowThumb
	}
	if s.SortBy != nil {
		data.SortBy = *s.SortBy
	}
	if s.SortDirection != nil {
		data.SortDirection = *s.SortDirection
	}
	if s.PageSize != nil {
		data.PageSize = *s.PageSize
	}
	if s.GalleryWidth != nil {
		data.GalleryWidth = *s.GalleryWidth
	}
	if s.ListColumns != nil {
		data.ListColumns = *s.ListColumns
	}

	return SetFolderViewPreference(c, path, &data)
}
