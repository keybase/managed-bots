package base

import (
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
)

type multi struct {
	sync.Mutex
	*DebugOutput

	db             *DB
	id             string
	isLeader       bool
	timeoutSeconds int
	interval       time.Duration
}

func newMulti(kbc *kbchat.API, db *DB, debugConfig *ChatDebugOutputConfig) *multi {
	return &multi{
		DebugOutput:    NewDebugOutput("Multi", debugConfig),
		db:             db,
		timeoutSeconds: 5,
		interval:       time.Second,
	}
}

func (m *multi) Heartbeat(shutdownCh chan struct{}) (err error) {
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
	m.Lock()
	defer m.Unlock()
	return m.isLeader
}

func (m *multi) heartbeat() {
	if err := m.db.RunTxn(func(tx *sql.Tx) error {
		// update ourselves first
		_, err := m.db.Exec(`
			INSERT INTO heartbeats (id, mtime) VALUES (?, NOW(6)) ON DUPLICATE KEY UPDATE mtime=NOW(6)
		`, m.id)
		if err != nil {
			m.Errorf("failed to register heartbeat: %s", err)
			return err
		}
		// see if we are the leader
		rows, err := m.db.Query(fmt.Sprintf(`
			SELECT id FROM heartbeats WHERE mtime > NOW(6) - %d SECOND
		`, m.timeoutSeconds))
		if err != nil {
			m.Errorf("failed to select heartbeaters: %s", err)
			return err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				m.Errorf("failed to scan id: %s", err)
				return err
			}
			ids = append(ids, id)
		}

		// figure out if we are the leader
		sort.Strings(ids)
		m.Lock()
		lastLeader := m.isLeader
		m.isLeader = ids[0] == m.id
		if lastLeader != m.isLeader {
			m.Debug("heartbeat: leader change: isLeader: %v", m.isLeader)
		}
		m.Unlock()
		return nil
	}); err != nil {
		m.Debug("heartbeat failed to run txn: %s", err)
	}
}

func (m *multi) deregister() {
	if err := m.db.RunTxn(func(tx *sql.Tx) error {
		_, err := m.db.Exec(`
			DELETE from heartbeats WHERE id = ?
		`, m.id)
		return err
	}); err != nil {
		m.Debug("deregister: failed to run txn: %s", err)
	}
}
