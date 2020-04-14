package base

import (
	"fmt"
	"strings"
	"time"

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

func (d *DebugOutput) Debug(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s: %s\n", d.name, msg)
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
		if err := GetNonFatalChatError(err); err != nil {
			d.Debug("ChatEcho: failed to send echo message: %s", err)
			return
		}
		d.Errorf("ChatEcho: failed to send echo message: %s", err)
	}
}

func (d *DebugOutput) Trace(err *error, format string, args ...interface{}) func() {
	msg := fmt.Sprintf(format, args...)
	start := time.Now()
	fmt.Printf("+ %s: %s\n", d.name, msg)
	return func() {
		fmt.Printf("- %s: %s -> %s [time=%v]\n", d.name, msg, ErrToOK(err), time.Since(start))
	}
}

func GetNonFatalChatError(err error) error {
	// error created in https://github.com/keybase/client/blob/1985b18c4e7659bede1d4a2e68e4f68467acebc6/go/client/chat_svc_handler.go#L1407
	// error created in https://github.com/keybase/keybase/blob/9a82c96231ea2c6132532002e58bac80849265e6/go/chatbase/storage/sql_chat.go#L2324
	if strings.Contains(err.Error(), "no conversations matched") ||
		strings.Contains(err.Error(), "GetConvTriple called with unknown ConversationID") {
		return err
	}
	return nil
}
