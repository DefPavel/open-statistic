package database

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"open-statistic/internal/parser"
)

type DB struct {
	conn        *sql.DB
	userCache   map[string]int64
	userCacheMu sync.RWMutex
}

// New создаёт подключение к SQLite
func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_cache_size=-64000&_temp_store=MEMORY")
	if err != nil {
		return nil, fmt.Errorf("открытие БД: %w", err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	db := &DB{conn: conn, userCache: make(map[string]int64)}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("миграция: %w", err)
	}
	return db, nil
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		common_name TEXT NOT NULL UNIQUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS traffic_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		real_address TEXT,
		virtual_address TEXT,
		bytes_received BIGINT NOT NULL DEFAULT 0,
		bytes_sent BIGINT NOT NULL DEFAULT 0,
		connected_since DATETIME,
		snapshot_at DATETIME NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);
	CREATE INDEX IF NOT EXISTS idx_traffic_snapshot_at ON traffic_snapshots(snapshot_at);
	CREATE INDEX IF NOT EXISTS idx_traffic_user ON traffic_snapshots(user_id);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_users_common_name ON users(common_name);
	`
	_, err := db.conn.Exec(schema)
	return err
}

// SaveSnapshot сохраняет снимок статистики из status-файла
func (db *DB) SaveSnapshot(status *parser.Status) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	snapshotAt := status.UpdatedAt
	if snapshotAt.IsZero() {
		snapshotAt = time.Now().UTC()
	}

	insert, err := tx.Prepare(`INSERT INTO traffic_snapshots (user_id, real_address, virtual_address, bytes_received, bytes_sent, connected_since, snapshot_at) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insert.Close()

	for _, c := range status.Clients {
		userID, err := db.ensureUser(tx, c.CommonName)
		if err != nil {
			return err
		}
		if _, err := insert.Exec(userID, c.RealAddress, c.VirtualAddr, c.BytesReceived, c.BytesSent, c.ConnectedSince, snapshotAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) ensureUser(tx *sql.Tx, commonName string) (int64, error) {
	db.userCacheMu.RLock()
	if id, ok := db.userCache[commonName]; ok {
		db.userCacheMu.RUnlock()
		return id, nil
	}
	db.userCacheMu.RUnlock()

	var id int64
	err := tx.QueryRow("SELECT id FROM users WHERE common_name = ?", commonName).Scan(&id)
	if err == nil {
		db.userCacheMu.Lock()
		db.userCache[commonName] = id
		db.userCacheMu.Unlock()
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	res, err := tx.Exec("INSERT INTO users (common_name) VALUES (?)", commonName)
	if err != nil {
		return 0, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return 0, err
	}
	db.userCacheMu.Lock()
	db.userCache[commonName] = id
	db.userCacheMu.Unlock()
	return id, nil
}

// UserTraffic статистика пользователя
type UserTraffic struct {
	CommonName    string `json:"common_name"`
	BytesReceived int64  `json:"bytes_received"`
	BytesSent     int64  `json:"bytes_sent"`
	TotalBytes    int64  `json:"total_bytes"`
}

var maxSnapshotQuery = `SELECT MAX(snapshot_at) FROM traffic_snapshots`

// GetUsers возвращает список пользователей
func (db *DB) GetUsers() ([]string, error) {
	rows, err := db.conn.Query("SELECT common_name FROM users ORDER BY common_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]string, 0, 32)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		users = append(users, name)
	}
	return users, rows.Err()
}

// GetUserTraffic возвращает трафик пользователя из последнего снимка
func (db *DB) GetUserTraffic(commonName string) (*UserTraffic, error) {
	var ut UserTraffic
	err := db.conn.QueryRow(`
		SELECT u.common_name, COALESCE(t.bytes_received, 0), COALESCE(t.bytes_sent, 0)
		FROM users u
		LEFT JOIN traffic_snapshots t ON u.id = t.user_id AND t.snapshot_at = (`+maxSnapshotQuery+`)
		WHERE u.common_name = ?`, commonName).Scan(&ut.CommonName, &ut.BytesReceived, &ut.BytesSent)
	if err != nil {
		return nil, err
	}
	ut.TotalBytes = ut.BytesReceived + ut.BytesSent
	return &ut, nil
}

// GetAllTraffic возвращает трафик всех пользователей из последнего снимка
func (db *DB) GetAllTraffic() ([]UserTraffic, error) {
	rows, err := db.conn.Query(`
		SELECT u.common_name, COALESCE(t.bytes_received, 0), COALESCE(t.bytes_sent, 0)
		FROM users u
		LEFT JOIN traffic_snapshots t ON u.id = t.user_id AND t.snapshot_at = (` + maxSnapshotQuery + `)
		ORDER BY (COALESCE(t.bytes_received, 0) + COALESCE(t.bytes_sent, 0)) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]UserTraffic, 0, 32)
	for rows.Next() {
		var ut UserTraffic
		if err := rows.Scan(&ut.CommonName, &ut.BytesReceived, &ut.BytesSent); err != nil {
			return nil, err
		}
		ut.TotalBytes = ut.BytesReceived + ut.BytesSent
		result = append(result, ut)
	}
	return result, rows.Err()
}

// GetLatestSnapshot возвращает последний снимок (текущие подключения)
func (db *DB) GetLatestSnapshot() ([]parser.Client, error) {
	rows, err := db.conn.Query(`
		SELECT u.common_name, t.real_address, t.virtual_address, t.bytes_received, t.bytes_sent, t.connected_since
		FROM traffic_snapshots t
		JOIN users u ON u.id = t.user_id
		WHERE t.snapshot_at = (` + maxSnapshotQuery + `)
		ORDER BY u.common_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	clients := make([]parser.Client, 0, 32)
	for rows.Next() {
		var c parser.Client
		var connectedSince sql.NullTime
		if err := rows.Scan(&c.CommonName, &c.RealAddress, &c.VirtualAddr, &c.BytesReceived, &c.BytesSent, &connectedSince); err != nil {
			return nil, err
		}
		if connectedSince.Valid {
			c.ConnectedSince = connectedSince.Time
		}
		clients = append(clients, c)
	}
	return clients, rows.Err()
}

// CleanupOldSnapshots удаляет старые снимки, оставляя последние n
func (db *DB) CleanupOldSnapshots(keep int) error {
	if keep <= 0 {
		return nil
	}
	var cnt int64
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM traffic_snapshots").Scan(&cnt); err != nil || cnt <= int64(keep) {
		return err
	}
	_, err := db.conn.Exec(`DELETE FROM traffic_snapshots WHERE snapshot_at < (
		SELECT MIN(s) FROM (SELECT snapshot_at AS s FROM traffic_snapshots ORDER BY snapshot_at DESC LIMIT ?)
	)`, keep)
	return err
}

// Close закрывает соединение
func (db *DB) Close() error {
	return db.conn.Close()
}
