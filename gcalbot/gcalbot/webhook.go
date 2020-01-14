package gcalbot

import (
	"sync"

	"google.golang.org/api/calendar/v3"
)

type WebhookChannel struct {
	Username        string
	Nickname        string
	CalendarID      string
	NextSyncToken   string
	CalendarService *calendar.Service
}

type WebhookChannels struct {
	channelMap sync.Map
}

func (c *WebhookChannels) Get(channelID string) (*WebhookChannel, bool) {
	val, ok := c.channelMap.Load(channelID)
	if ok {
		return val.(*WebhookChannel), true
	} else {
		return nil, false
	}
}

func (c *WebhookChannels) Set(channelID string, account *WebhookChannel) {
	c.channelMap.Store(channelID, account)
}

func (c *WebhookChannels) Delete(channelID string) {
	c.channelMap.Delete(channelID)
}
