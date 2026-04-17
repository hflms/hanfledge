package postgres

import (
	"context"
	"testing"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupMarketplaceTestDB(t *testing.T) (*gorm.DB, *MarketplaceRepo) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&model.MarketplacePlugin{},
		&model.MarketplaceReview{},
		&model.InstalledPlugin{},
	)
	require.NoError(t, err)

	repo := NewMarketplaceRepo(db)
	return db, repo
}

func TestNewMarketplaceRepo(t *testing.T) {
	db, repo := setupMarketplaceTestDB(t)
	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.DB)
}

func TestEscapeLike(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special chars",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "percentage sign",
			input:    "100%",
			expected: "100\\%",
		},
		{
			name:     "underscore",
			input:    "my_plugin",
			expected: "my\\_plugin",
		},
		{
			name:     "backslash",
			input:    "C:\\path",
			expected: "C:\\\\path",
		},
		{
			name:     "mixed special chars",
			input:    "100%_my\\plugin",
			expected: "100\\%\\_my\\\\plugin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeLike(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateAndFindPlugin(t *testing.T) {
	_, repo := setupMarketplaceTestDB(t)
	ctx := context.Background()

	plugin := &model.MarketplacePlugin{
		PluginID: "test.plugin.1",
		Name:     "Test Plugin",
		Version:  "1.0.0",
		Type:     "skill",
		Status:   "pending",
	}

	// Test CreatePlugin
	err := repo.CreatePlugin(ctx, plugin)
	require.NoError(t, err)
	assert.NotZero(t, plugin.ID)

	// Test FindByPluginID
	found, err := repo.FindByPluginID(ctx, "test.plugin.1")
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, "Test Plugin", found.Name)

	// Test FindByPluginID - Not Found
	notFound, err := repo.FindByPluginID(ctx, "non.existent")
	assert.Error(t, err)
	assert.Nil(t, notFound)
	assert.Equal(t, gorm.ErrRecordNotFound, err)

	// Test FindApprovedByPluginID - Pending status should not be found
	notApproved, err := repo.FindApprovedByPluginID(ctx, "test.plugin.1")
	assert.Error(t, err)
	assert.Nil(t, notApproved)
	assert.Equal(t, gorm.ErrRecordNotFound, err)

	// Update to approved and test again
	plugin.Status = "approved"
	repo.DB.Save(plugin)

	approved, err := repo.FindApprovedByPluginID(ctx, "test.plugin.1")
	require.NoError(t, err)
	assert.NotNil(t, approved)
	assert.Equal(t, "approved", approved.Status)
}

func TestListApprovedAndIncrementDownloads(t *testing.T) {
	_, repo := setupMarketplaceTestDB(t)
	ctx := context.Background()

	plugins := []model.MarketplacePlugin{
		{PluginID: "p1", Name: "Math Tool", Category: "math", Type: "skill", Status: "approved", Downloads: 10},
		{PluginID: "p2", Name: "Physics Tool", Category: "science", Type: "skill", Status: "approved", Downloads: 5},
		{PluginID: "p3", Name: "Draft Tool", Category: "math", Type: "skill", Status: "pending", Downloads: 0},
		{PluginID: "p4", Name: "Math Solver 100%", Description: "A great app", Category: "math", Type: "editor", Status: "approved", Downloads: 20},
	}

	for _, p := range plugins {
		err := repo.CreatePlugin(ctx, &p)
		require.NoError(t, err)
	}

	tests := []struct {
		name          string
		pluginType    string
		category      string
		search        string
		offset        int
		limit         int
		expectedCount int
		expectedTotal int64
		firstPluginID string // highest downloads first
	}{
		{
			name:          "all approved",
			pluginType:    "",
			category:      "",
			search:        "",
			offset:        0,
			limit:         10,
			expectedCount: 4,
			expectedTotal: 4,
			firstPluginID: "p4",
		},
		{
			name:          "filter by type",
			pluginType:    "skill",
			category:      "",
			search:        "",
			offset:        0,
			limit:         10,
			expectedCount: 3,
			expectedTotal: 3,
			firstPluginID: "p1",
		},
		{
			name:          "filter by category",
			pluginType:    "",
			category:      "math",
			search:        "",
			offset:        0,
			limit:         10,
			expectedCount: 2,
			expectedTotal: 2,
			firstPluginID: "p4",
		},
		{
			name:          "search by name",
			pluginType:    "",
			category:      "",
			search:        "Tool",
			offset:        0,
			limit:         10,
			expectedCount: 2,
			expectedTotal: 2,
			firstPluginID: "p1", // Tool doesn't match p4 "Math Solver 100%"
		},
		{
			name:          "pagination",
			pluginType:    "",
			category:      "",
			search:        "",
			offset:        1,
			limit:         1,
			expectedCount: 1,
			expectedTotal: 4,
			firstPluginID: "p1", // p4 is first, so offset 1 is p1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, total, err := repo.ListApproved(ctx, tt.pluginType, tt.category, tt.search, tt.offset, tt.limit)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTotal, total)
			assert.Len(t, results, tt.expectedCount)
			if tt.expectedCount > 0 {
				assert.Equal(t, tt.firstPluginID, results[0].PluginID)
			}
		})
	}

	// Test IncrementDownloads
	// get ID of p4
	var p4 model.MarketplacePlugin
	err := repo.DB.Where("plugin_id = ?", "p4").First(&p4).Error
	require.NoError(t, err)
	assert.Equal(t, 20, p4.Downloads)

	err = repo.IncrementDownloads(ctx, p4.ID)
	require.NoError(t, err)

	var p4Updated model.MarketplacePlugin
	err = repo.DB.Where("plugin_id = ?", "p4").First(&p4Updated).Error
	require.NoError(t, err)
	assert.Equal(t, 21, p4Updated.Downloads)
}

func TestInstalledPlugins(t *testing.T) {
	_, repo := setupMarketplaceTestDB(t)
	ctx := context.Background()

	installed1 := &model.InstalledPlugin{
		SchoolID: 1,
		PluginID: "plugin.1",
		Version:  "1.0.0",
		Enabled:  true,
	}

	installed2 := &model.InstalledPlugin{
		SchoolID: 1,
		PluginID: "plugin.2",
		Version:  "1.0.0",
		Enabled:  false,
	}

	installed3 := &model.InstalledPlugin{
		SchoolID: 2,
		PluginID: "plugin.1",
		Version:  "1.0.0",
		Enabled:  true,
	}

	// Test CreateInstalledPlugin
	err := repo.CreateInstalledPlugin(ctx, installed1)
	require.NoError(t, err)
	assert.NotZero(t, installed1.ID)

	err = repo.CreateInstalledPlugin(ctx, installed2)
	require.NoError(t, err)

	err = repo.CreateInstalledPlugin(ctx, installed3)
	require.NoError(t, err)

	// Test FindInstalledPlugin
	found, err := repo.FindInstalledPlugin(ctx, 1, "plugin.1")
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, installed1.ID, found.ID)
	assert.True(t, found.Enabled)

	// Test FindInstalledPlugin - Not Found
	notFound, err := repo.FindInstalledPlugin(ctx, 1, "non.existent")
	assert.Error(t, err)
	assert.Nil(t, notFound)
	assert.Equal(t, gorm.ErrRecordNotFound, err)

	// Test ListInstalledBySchool
	school1Plugins, err := repo.ListInstalledBySchool(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, school1Plugins, 2)

	school2Plugins, err := repo.ListInstalledBySchool(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, school2Plugins, 1)
	assert.Equal(t, "plugin.1", school2Plugins[0].PluginID)

	school3Plugins, err := repo.ListInstalledBySchool(ctx, 3)
	require.NoError(t, err)
	assert.Len(t, school3Plugins, 0)

	// Test DeleteInstalledPlugin
	err = repo.DeleteInstalledPlugin(ctx, installed2.ID)
	require.NoError(t, err)

	school1PluginsAfterDelete, err := repo.ListInstalledBySchool(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, school1PluginsAfterDelete, 1)
	assert.Equal(t, "plugin.1", school1PluginsAfterDelete[0].PluginID)
}

func TestListReviewsByPluginID(t *testing.T) {
	_, repo := setupMarketplaceTestDB(t)
	ctx := context.Background()

	reviews := []model.MarketplaceReview{
		{PluginID: "plugin.1", UserID: 1, Rating: 5, Comment: "Great!"},
		{PluginID: "plugin.1", UserID: 2, Rating: 4, Comment: "Good"},
		{PluginID: "plugin.1", UserID: 3, Rating: 3, Comment: "Okay"},
		{PluginID: "plugin.2", UserID: 1, Rating: 5, Comment: "Awesome"},
	}

	for _, r := range reviews {
		err := repo.DB.WithContext(ctx).Create(&r).Error
		require.NoError(t, err)
	}

	// Test ListReviewsByPluginID
	results, err := repo.ListReviewsByPluginID(ctx, "plugin.1", 2)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// They should be ordered by created_at DESC, so the last inserted should be first.
	// But in SQLite with same millisecond timestamps, order might be non-deterministic if not strictly increasing
	// At least we check the lengths

	allResults, err := repo.ListReviewsByPluginID(ctx, "plugin.1", 10)
	require.NoError(t, err)
	assert.Len(t, allResults, 3)

	noResults, err := repo.ListReviewsByPluginID(ctx, "non.existent", 10)
	require.NoError(t, err)
	assert.Len(t, noResults, 0)
}
