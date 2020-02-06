package elastiwatch

import (
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
)

type HTTPSrv struct {
	*base.HTTPSrv

	kbc *kbchat.API
	db  *DB
}

func NewHTTPSrv(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig, db *DB) *HTTPSrv {
	return &HTTPSrv{
		HTTPSrv: base.NewHTTPSrv(stats, debugConfig),
		kbc:     kbc,
		db:      db,
	}
}
