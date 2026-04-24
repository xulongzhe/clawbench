package service_test

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"clawbench/internal/service"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/assert"
)

const recentProjectsSchema = `
CREATE TABLE IF NOT EXISTS recent_projects (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_path TEXT UNIQUE NOT NULL,
	accessed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

func setupRecentProjectsDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	assert.NoError(t, err)

	_, err = db.Exec(recentProjectsSchema)
	assert.NoError(t, err)

	service.DB = db
	t.Cleanup(func() {
		db.Close()
	})
	return db
}

// insertProjectWithTime inserts a project with an explicit accessed_at timestamp
// to ensure deterministic ordering in tests (CURRENT_TIMESTAMP has only second precision).
func insertProjectWithTime(t *testing.T, db *sql.DB, path string, accessedAt time.Time) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO recent_projects (project_path, accessed_at) VALUES (?, ?)",
		path, accessedAt.Format("2006-01-02 15:04:05"),
	)
	assert.NoError(t, err)
}

func TestGetRecentProjects_Empty(t *testing.T) {
	setupRecentProjectsDB(t)

	paths, err := service.GetRecentProjects()
	assert.NoError(t, err)
	assert.Empty(t, paths)
}

func TestAddRecentProject(t *testing.T) {
	setupRecentProjectsDB(t)

	err := service.AddRecentProject("/home/user/project1")
	assert.NoError(t, err)

	paths, err := service.GetRecentProjects()
	assert.NoError(t, err)
	assert.Equal(t, []string{"/home/user/project1"}, paths)
}

func TestGetRecentProjects_OrderedByAccessedAtDesc(t *testing.T) {
	db := setupRecentProjectsDB(t)

	// Insert with explicit timestamps to guarantee ordering
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	insertProjectWithTime(t, db, "/project/a", baseTime)
	insertProjectWithTime(t, db, "/project/b", baseTime.Add(1*time.Second))
	insertProjectWithTime(t, db, "/project/c", baseTime.Add(2*time.Second))

	paths, err := service.GetRecentProjects()
	assert.NoError(t, err)
	assert.Len(t, paths, 3)

	// Most recently accessed should be first
	assert.Equal(t, "/project/c", paths[0])
	assert.Equal(t, "/project/b", paths[1])
	assert.Equal(t, "/project/a", paths[2])
}

func TestAddRecentProject_Upsert(t *testing.T) {
	db := setupRecentProjectsDB(t)

	// Insert with explicit timestamps so the upsert bump is testable
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	insertProjectWithTime(t, db, "/project/a", baseTime)
	insertProjectWithTime(t, db, "/project/b", baseTime.Add(1*time.Second))

	// Re-add project/a via the service function - should update timestamp
	err := service.AddRecentProject("/project/a")
	assert.NoError(t, err)

	paths, err := service.GetRecentProjects()
	assert.NoError(t, err)
	assert.Len(t, paths, 2) // Still only 2 entries, not 3

	// project/a should now be the most recent (its timestamp was just updated)
	assert.Equal(t, "/project/a", paths[0])
	assert.Equal(t, "/project/b", paths[1])
}

func TestAddRecentProject_PruneBeyond10(t *testing.T) {
	setupRecentProjectsDB(t)

	// Add 12 projects via the service (timestamps will be close but pruning still works)
	for i := 0; i < 12; i++ {
		err := service.AddRecentProject(fmt.Sprintf("/project/%02d", i))
		assert.NoError(t, err)
	}

	paths, err := service.GetRecentProjects()
	assert.NoError(t, err)
	assert.Len(t, paths, 10) // Should be capped at 10
}

func TestGetRecentProjects_Limit10(t *testing.T) {
	setupRecentProjectsDB(t)

	// Add exactly 10 projects
	for i := 0; i < 10; i++ {
		err := service.AddRecentProject(fmt.Sprintf("/project/%02d", i))
		assert.NoError(t, err)
	}

	paths, err := service.GetRecentProjects()
	assert.NoError(t, err)
	assert.Len(t, paths, 10)
}

func TestAddRecentProject_PruneKeepsMostRecent(t *testing.T) {
	db := setupRecentProjectsDB(t)

	// Insert 10 projects with explicit timestamps
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		insertProjectWithTime(t, db, fmt.Sprintf("/project/%02d", i), baseTime.Add(time.Duration(i)*time.Second))
	}

	// Add an 11th via service - should prune the oldest (/project/00)
	err := service.AddRecentProject("/project/new")
	assert.NoError(t, err)

	paths, err := service.GetRecentProjects()
	assert.NoError(t, err)
	assert.Len(t, paths, 10)

	// /project/00 should have been pruned (it had the oldest timestamp)
	for _, p := range paths {
		assert.NotEqual(t, "/project/00", p)
	}

	// /project/new should be present (it was just added)
	found := false
	for _, p := range paths {
		if p == "/project/new" {
			found = true
			break
		}
	}
	assert.True(t, found, "/project/new should be in the list")
}

func TestAddRecentProject_DuplicateDoesNotIncreaseCount(t *testing.T) {
	setupRecentProjectsDB(t)

	service.AddRecentProject("/project/a")
	service.AddRecentProject("/project/a")
	service.AddRecentProject("/project/a")

	paths, err := service.GetRecentProjects()
	assert.NoError(t, err)
	assert.Len(t, paths, 1) // Still just one entry for /project/a
}
