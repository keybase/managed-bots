package base

import (
	"fmt"
	"strings"

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

func (d *DebugOutput) Config() *ChatDebugOutputConfig {
	return d.config
}

func (d *DebugOutput) Debug(msg string, args ...interface{}) {
	fmt.Printf(d.name+": "+msg+"\n", args...)
}

func (d *DebugOutput) Errorf(msg string, args ...interface{}) {
	d.Debug(msg, args...)
	msg = fmt.Sprintf("```%s```", msg)
	d.Report(msg, args...)
}

func (d *DebugOutput) Report(msg string, args ...interface{}) {
	if d.config == nil {
		d.Debug("Errorf: Unable to report error to chat, errReportConv, chat debug not configured")
	} else if d.config.ErrReportConv == "" || d.config.KBC == nil {
		d.Debug("Errorf: Unable to report error to chat, errReportConv: %v, kbc: %v",
			d.config.ErrReportConv, d.config.KBC)
	} else {
		if err := SendByConvNameOrID(d.config.KBC, d, d.config.ErrReportConv, msg, args...); err != nil {
			d.Debug("Errorf: failed to send error message: %s", err)
		}
	}
}

func (d *DebugOutput) ChatDebug(convID chat1.ConvIDStr, msg string, args ...interface{}) {
	d.Debug(msg, args...)
	if _, err := d.config.KBC.SendMessageByConvID(convID, "Something went wrong!"); err != nil {
		d.Errorf("ChatDebug: failed to send error message: %s", err)
	}
}

func (d *DebugOutput) ChatErrorf(convID chat1.ConvIDStr, msg string, args ...interface{}) {
	d.Errorf(msg, args...)
	if _, err := d.config.KBC.SendMessageByConvID(convID, "Something went wrong!"); err != nil {
		d.Errorf("ChatErrorf: failed to send error message: %s", err)
	}
}

func (d *DebugOutput) ChatEcho(convID chat1.ConvIDStr, msg string, args ...interface{}) {
	if _, err := d.config.KBC.SendMessageByConvID(convID, msg, args...); err != nil {
		// error created in https://github.com/keybase/client/blob/1985b18c4e7659bede1d4a2e68e4f68467acebc6/go/client/chat_svc_handler.go#L1407
		if strings.HasPrefix(err.Error(), "no conversations matched") {
			d.Debug("ChatEcho: failed to send echo message: %s", err)
			return
		}
		d.Errorf("ChatEcho: failed to send echo message: %s", err)
	}
}
