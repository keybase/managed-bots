package base

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

type multi struct {
	sync.Mutex
	*DebugOutput

	db             *DB
	name           string
	id             string
	isLeader       bool
	timeoutSeconds int
	interval       time.Duration
}

func newMulti(name string, db *DB, debugConfig *ChatDebugOutputConfig) *multi {
	return &multi{
		DebugOutput:    NewDebugOutput("Multi", debugConfig),
		db:             db,
		timeoutSeconds: 5,
		interval:       time.Second,
		name:           name,
	}
}

func (m *multi) Heartbeat(shutdownCh chan struct{}) (err error) {
	if m == nil {
		return nil
	}
	defer m.Trace(&err, "Heartbeat")()
	m.id = RandHexString(8)
	m.Debug("Heartbeat: starting multi coordination heartbeat loop: id: %s", m.id)
	for {
		select {
		case <-time.After(m.interval):
			m.heartbeat()
		case <-shutdownCh:
			m.Debug("Heartbeat: shutdown received, deregistering")
			m.deregister()
			return nil
		}
	}
}

func (m *multi) IsLeader() bool {
	if m == nil {
		return true
	}
	m.Lock()
	defer m.Unlock()
	return m.isLeader
}

func (m *multi) heartbeat() {
	// update ourselves first
	err := m.db.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO heartbeats (id, name, mtime)
			VALUES (?, ?, NOW(6)) ON DUPLICATE KEY UPDATE mtime=NOW(6)
		`, m.id, m.name)
		return err
	})
	if err != nil {
		m.Errorf("failed to register heartbeat tx: %s", err)
		return
	}
	// see if we are the leader
	row := m.db.QueryRow(fmt.Sprintf(`
			SELECT id FROM heartbeats
			WHERE mtime > NOW(6) - INTERVAL %d SECOND AND name = ?
			ORDER BY id DESC
			LIMIT 1
		`, m.timeoutSeconds), m.name)
	var id string
	if err := row.Scan(&id); err != nil {
		if err != sql.ErrNoRows {
			m.Errorf("failed to scan id: %s", err)
			return
		}
	}

	// figure out if we are the leader
	m.Lock()
	defer m.Unlock()
	lastLeader := m.isLeader
	m.isLeader = id == m.id
	if lastLeader != m.isLeader {
		if m.isLeader {
			m.Errorf("heartbeat: leader change: isLeader: %v myid: %s", m.isLeader, m.id)
		} else {
			m.Errorf("heartbeat: leader change: isLeader: %v myid: %s leaderid: %s", m.isLeader, m.id,
				id)
		}
	}
}

func (m *multi) deregister() {
	err := m.db.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE from heartbeats
			WHERE id = ? OR mtime < NOW() - INTERVAL 1 MINUTE
		`, m.id)
		return err
	})
	if err != nil {
		m.Errorf("deregister: failed to execute : %s", err)
	}
}
