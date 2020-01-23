package base

import (
	"fmt"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type ChatDebugOutputConfig struct {
	KBC           *kbchat.API
	ErrReportConv string
}

func NewChatDebugOutputConfig(kbc *kbchat.API, errReportConv string) *ChatDebugOutputConfig {
	return &ChatDebugOutputConfig{
		KBC:           kbc,
		ErrReportConv: errReportConv,
	}
}

type DebugOutput struct {
	config *ChatDebugOutputConfig
	name   string
}

func NewDebugOutput(name string, config *ChatDebugOutputConfig) *DebugOutput {
	return &DebugOutput{
		name:   name,
		config: config,
	}
}

func (d *DebugOutput) Debug(msg string, args ...interface{}) {
	fmt.Printf(d.name+": "+msg+"\n", args...)
}

func (d *DebugOutput) Error(msg string, args ...interface{}) {
	d.Debug(msg, args...)
	if d.config == nil {
		d.Debug("Unable to report error to chat, errReportConv, chat debug not configured")
	} else if d.config.ErrReportConv == "" || d.config.KBC == nil {
		d.Debug("Unable to report error to chat, errReportConv: %v, kbc: %v",
			d.config.ErrReportConv, d.config.KBC)
	} else {
		if err := SendByConvNameOrID(d.config.KBC, d.config.ErrReportConv, msg, args...); err != nil {
			d.Debug("Error: failed to send error message: %s", err)
		}
	}
}

func (d *DebugOutput) ChatDebug(convID chat1.ConvIDStr, msg string, args ...interface{}) {
	d.Debug(msg, args...)
	if _, err := d.config.KBC.SendMessageByConvID(convID, "Something went wrong!"); err != nil {
		d.Debug("chatDebug: failed to send error message: %s", err)
	}
}

func (d *DebugOutput) ChatDebugFull(convID chat1.ConvIDStr, msg string, args ...interface{}) {
	d.Debug(msg, args...)
	if _, err := d.config.KBC.SendMessageByConvID(convID, msg, args...); err != nil {
		d.Debug("chatDebugFull: failed to send error message: %s", err)
	}
}

func (d *DebugOutput) ChatEcho(convID chat1.ConvIDStr, msg string, args ...interface{}) {
	if _, err := d.config.KBC.SendMessageByConvID(convID, msg, args...); err != nil {
		d.Debug("chatEcho: failed to send echo message: %s", err)
	}
}
