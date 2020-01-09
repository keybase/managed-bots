package base

import (
	"fmt"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type DebugOutput struct {
	name string
	kbc  *kbchat.API
}

func NewDebugOutput(name string, kbc *kbchat.API) *DebugOutput {
	return &DebugOutput{
		name: name,
		kbc:  kbc,
	}
}

func (d *DebugOutput) Debug(msg string, args ...interface{}) {
	fmt.Printf(d.name+": "+msg+"\n", args...)
}

func (d *DebugOutput) ChatDebug(convID chat1.APIConvID, msg string, args ...interface{}) {
	d.Debug(msg, args...)
	if _, err := d.kbc.SendMessageByConvID(convID, "Something went wrong!"); err != nil {
		d.Debug("chatDebug: failed to send error message: %s", err)
	}
}

func (d *DebugOutput) ChatDebugFull(convID chat1.APIConvID, msg string, args ...interface{}) {
	d.Debug(msg, args...)
	if _, err := d.kbc.SendMessageByConvID(convID, msg, args...); err != nil {
		d.Debug("chatDebug: failed to send error message: %s", err)
	}
}

func (d *DebugOutput) ChatEcho(convID chat1.APIConvID, msg string, args ...interface{}) {
	if _, err := d.kbc.SendMessageByConvID(convID, msg, args...); err != nil {
		d.Debug("chatEcho: failed to send echo message: %s", err)
	}
}
