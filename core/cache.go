package core

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Cache struct {
	db *sql.DB
}

// OpenCache opens or creates the local SQLite cache
func OpenCache() (*Cache, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = filepath.Join(os.TempDir(), "autoxmate-cache")
	}
	dir := filepath.Join(cacheDir, "autoxmate")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	dbPath := filepath.Join(dir, "autoxmate.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open cache db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping cache db: %w", err)
	}

	c := &Cache{db: db}
	if err := c.migrate(); err != nil {
		return nil, fmt.Errorf("migrate cache: %w", err)
	}

	return c, nil
}

func (c *Cache) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS tools (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			namespace TEXT,
			data TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS install_status (
			tool_id TEXT PRIMARY KEY,
			installed INTEGER NOT NULL DEFAULT 0,
			on_path INTEGER NOT NULL DEFAULT 0,
			version TEXT,
			path_location TEXT,
			checked_at TEXT NOT NULL,
			FOREIGN KEY (tool_id) REFERENCES tools(id)
		)`,
		`CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS queries_cache (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query_string TEXT UNIQUE NOT NULL,
			result_json TEXT NOT NULL,
			executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			executed_count INTEGER DEFAULT 0
		)`,
	}
	for _, q := range queries {
		if _, err := c.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// StoreTools caches tool definitions
func (c *Cache) StoreTools(tools []ToolDefinition) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO tools (id, name, namespace, data, updated_at) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, t := range tools {
		data, err := json.Marshal(t)
		if err != nil {
			return err
		}
		if _, err := stmt.Exec(t.ID, t.Name, t.Namespace, string(data), now); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// LoadTools reads all cached tool definitions
func (c *Cache) LoadTools() ([]ToolDefinition, error) {
	rows, err := c.db.Query(`SELECT data FROM tools ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tools []ToolDefinition
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var t ToolDefinition
		if err := json.Unmarshal([]byte(data), &t); err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}

// UpdateInstallStatus updates detection results for a tool
func (c *Cache) UpdateInstallStatus(toolID string, status InstallStatus) error {
	now := time.Now().UTC().Format(time.RFC3339)
	onPath := 0
	if status.OnPath {
		onPath = 1
	}
	installed := 0
	if status.OnPath || status.DockerImage || status.PackageManager != "" {
		installed = 1
	}
	_, err := c.db.Exec(
		`INSERT OR REPLACE INTO install_status (tool_id, installed, on_path, version, path_location, checked_at) VALUES (?, ?, ?, ?, ?, ?)`,
		toolID, installed, onPath, status.Version, status.PathLocation, now,
	)
	return err
}

// GetLastSync returns the last sync timestamp
func (c *Cache) GetLastSync() (time.Time, error) {
	var val string
	err := c.db.QueryRow(`SELECT value FROM metadata WHERE key = 'last_sync'`).Scan(&val)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, val)
}

// SetLastSync stores the last sync timestamp
func (c *Cache) SetLastSync(t time.Time) error {
	_, err := c.db.Exec(
		`INSERT OR REPLACE INTO metadata (key, value) VALUES ('last_sync', ?)`,
		t.UTC().Format(time.RFC3339),
	)
	return err
}

// SaveQueryResult caches a query and its result
func (c *Cache) SaveQueryResult(queryString string, results []QueryResult) error {
	data, err := json.Marshal(results)
	if err != nil {
		return err
	}
	_, err = c.db.Exec(
		`INSERT INTO queries_cache (query_string, result_json, executed_count)
		 VALUES (?, ?, 1)
		 ON CONFLICT(query_string) DO UPDATE SET
		   result_json = excluded.result_json,
		   executed_at = CURRENT_TIMESTAMP,
		   executed_count = executed_count + 1`,
		queryString, string(data),
	)
	return err
}

// LoadQueryResult loads a cached query result
func (c *Cache) LoadQueryResult(queryString string) ([]QueryResult, error) {
	var data string
	err := c.db.QueryRow(`SELECT result_json FROM queries_cache WHERE query_string = ?`, queryString).Scan(&data)
	if err != nil {
		return nil, err
	}
	var results []QueryResult
	if err := json.Unmarshal([]byte(data), &results); err != nil {
		return nil, err
	}
	return results, nil
}

// StoreQueriesIndex caches the queries.json data
func (c *Cache) StoreQueriesIndex(idx *QueriesIndex) error {
	data, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	_, err = c.db.Exec(
		`INSERT OR REPLACE INTO metadata (key, value) VALUES ('queries_index', ?)`,
		string(data),
	)
	return err
}

// LoadQueriesIndex loads the cached queries index
func (c *Cache) LoadQueriesIndex() (*QueriesIndex, error) {
	var data string
	err := c.db.QueryRow(`SELECT value FROM metadata WHERE key = 'queries_index'`).Scan(&data)
	if err != nil {
		return nil, err
	}
	var idx QueriesIndex
	if err := json.Unmarshal([]byte(data), &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// Close closes the database connection
func (c *Cache) Close() error {
	return c.db.Close()
}

// ToolCount returns the number of tools in cache
func (c *Cache) ToolCount() (int, error) {
	var count int
	err := c.db.QueryRow(`SELECT COUNT(*) FROM tools`).Scan(&count)
	return count, err
}
