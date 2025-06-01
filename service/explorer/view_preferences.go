package explorer

import (
	"context"

	"github.com/cloudreve/Cloudreve/v4/application/dependency"
	"github.com/cloudreve/Cloudreve/v4/ent"
	"github.com/cloudreve/Cloudreve/v4/inventory"
	"github.com/cloudreve/Cloudreve/v4/inventory/types"
	"github.com/cloudreve/Cloudreve/v4/pkg/filemanager/fs"
	"github.com/cloudreve/Cloudreve/v4/pkg/filemanager/fs/dbfs"
	"github.com/cloudreve/Cloudreve/v4/pkg/filemanager/manager"
	"github.com/cloudreve/Cloudreve/v4/pkg/serializer"
	"github.com/gin-gonic/gin"
)

type (
	// UpdateViewPreferencesService updates view preferences for a folder
	UpdateViewPreferencesService struct {
		Uri             string                 `json:"uri" form:"uri" binding:"required"`
		ViewPreferences *types.ViewPreferences `json:"view_preferences" form:"view_preferences" binding:"required"`
	}

	// GetViewPreferencesService gets view preferences for a folder
	GetViewPreferencesService struct {
		Uri string `json:"uri" form:"uri" binding:"required"`
	}

	// UpdateDefaultViewPreferencesService updates default view preferences for user
	UpdateDefaultViewPreferencesService struct {
		DefaultViewMode  *string           `json:"default_view_mode"`
		DefaultSortBy    *string           `json:"default_sort_by"`
		DefaultSortOrder *string           `json:"default_sort_order"`
		ViewPreferences  map[string]string `json:"view_preferences"`
	}
)

// UpdateViewPreferences updates view preferences for a specific folder
func (s *UpdateViewPreferencesService) Update(c *gin.Context) error {
	dep := dependency.FromContext(c)
	user := inventory.UserFromContext(c)
	m := manager.NewFileManager(dep, user)
	defer m.Recycle()

	uri, err := fs.NewUriFromString(s.Uri)
	if err != nil {
		return serializer.NewError(serializer.CodeParamErr, "unknown uri", err)
	}

	// Get the folder
	file, err := m.Get(c, uri, dbfs.WithNotRoot())
	if err != nil {
		return serializer.NewError(serializer.CodeIOFailed, "Failed to get folder", err)
	}

	// Check if it's a folder
	if file.Type() != types.FileTypeFolder {
		return serializer.NewError(serializer.CodeNotFound, "Target is not a folder", nil)
	}

	// Update view preferences
	props := file.(*dbfs.File).FileProps()
	if props == nil {
		props = &types.FileProps{}
	}
	props.ViewPreferences = s.ViewPreferences

	// Save changes
	fileClient := dep.FileClient()
	if err := fileClient.UpdateFileProps(c, file.ID(), props); err != nil {
		return serializer.NewError(serializer.CodeDBError, "Failed to update view preferences", err)
	}

	return nil
}

// GetViewPreferences gets effective view preferences for a folder
func (s *GetViewPreferencesService) Get(c *gin.Context) (*types.ViewPreferences, error) {
	dep := dependency.FromContext(c)
	user := inventory.UserFromContext(c)
	m := manager.NewFileManager(dep, user)
	defer m.Recycle()

	uri, err := fs.NewUriFromString(s.Uri)
	if err != nil {
		return nil, serializer.NewError(serializer.CodeParamErr, "unknown uri", err)
	}

	// Get the folder
	file, err := m.Get(c, uri, dbfs.WithNotRoot())
	if err != nil {
		return nil, serializer.NewError(serializer.CodeIOFailed, "Failed to get folder", err)
	}

	// Get effective view preferences (with inheritance)
	dbFile := file.(*dbfs.File)
	return getEffectiveViewPreferences(c, dep, user, dbFile)
}

// UpdateDefaultViewPreferences updates user's default view preferences
func (s *UpdateDefaultViewPreferencesService) Update(c *gin.Context) error {
	dep := dependency.FromContext(c)
	user := inventory.UserFromContext(c)
	userClient := dep.UserClient()

	// Update settings
	if s.DefaultViewMode != nil {
		user.Settings.DefaultViewMode = *s.DefaultViewMode
	}
	if s.DefaultSortBy != nil {
		user.Settings.DefaultSortBy = *s.DefaultSortBy
	}
	if s.DefaultSortOrder != nil {
		user.Settings.DefaultSortOrder = *s.DefaultSortOrder
	}
	if s.ViewPreferences != nil {
		user.Settings.ViewPreferences = s.ViewPreferences
	}

	// Save settings
	if err := userClient.SaveSettings(c, user); err != nil {
		return serializer.NewError(serializer.CodeDBError, "Failed to update default view preferences", err)
	}

	return nil
}

// getEffectiveViewPreferences gets the effective view preferences for a folder, considering inheritance
func getEffectiveViewPreferences(ctx context.Context, dep dependency.Dep, user *ent.User, file *dbfs.File) (*types.ViewPreferences, error) {
	props := file.FileProps()

	// If folder has explicit preferences, return them
	if props != nil && props.ViewPreferences != nil && props.ViewPreferences.InheritFrom == 0 {
		return props.ViewPreferences, nil
	}

	// Check if we should inherit from parent
	if props != nil && props.ViewPreferences != nil && props.ViewPreferences.InheritFrom > 0 {
		// Get parent folder
		fileClient := dep.FileClient()
		parent, err := fileClient.GetByID(ctx, props.ViewPreferences.InheritFrom)
		if err == nil && parent.OwnerID == user.ID {
			parentFile := &dbfs.File{}
			// Wrap the parent file to use getEffectiveViewPreferences recursively
			parentFile.SetFile(parent)
			return getEffectiveViewPreferences(ctx, dep, user, parentFile)
		}
	}

	// Check parent folder preferences
	if file.HasParent() {
		parentID := file.ParentID()
		fileClient := dep.FileClient()
		parent, err := fileClient.GetByID(ctx, parentID)
		if err == nil && parent.OwnerID == user.ID && parent.Props != nil && parent.Props.ViewPreferences != nil {
			parentFile := &dbfs.File{}
			parentFile.SetFile(parent)
			return parent.Props.ViewPreferences, nil
		}
	}

	// Return user's default preferences
	return &types.ViewPreferences{
		ViewMode:       user.Settings.DefaultViewMode,
		SortBy:         user.Settings.DefaultSortBy,
		SortOrder:      user.Settings.DefaultSortOrder,
		ShowThumb:      user.Settings.ViewPreferences != nil && user.Settings.ViewPreferences["show_thumb"] == "true",
		CustomSettings: user.Settings.ViewPreferences,
	}, nil
}
