package base

import (
	"fmt"
	"os"
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

func (d *DebugOutput) Debug(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s: %s\n", d.name, msg)
}

func (d *DebugOutput) Errorf(msg string, args ...any) {
	d.Debug(msg, args...)
	msg = fmt.Sprintf("```%s```", msg)
	d.Report(msg, args...)
}

func (d *DebugOutput) Report(msg string, args ...any) {
	if d.config == nil {
		d.Debug("Errorf: Unable to report error to chat, errReportConv, chat debug not configured")
	} else if d.config.ErrReportConv == "" || d.config.KBC == nil {
		d.Debug("Errorf: Unable to report error to chat, errReportConv: %v, kbc: %v",
			d.config.ErrReportConv, d.config.KBC)
	} else {
		if err := SendByConvNameOrID(d.config.KBC, d, d.config.ErrReportConv, msg, args...); err != nil && !IsIgnoredSendError(err) {
			d.Debug("Errorf: failed to send error message: %s", err)
		}
	}
}

func (d *DebugOutput) ChatDebug(convID chat1.ConvIDStr, msg string, args ...any) {
	d.Debug(msg, args...)
	if _, err := d.config.KBC.SendMessageByConvID(convID, "Something went wrong!"); err != nil && !IsIgnoredSendError(err) {
		d.Errorf("ChatDebug: failed to send error message: %s", err)
	}
}

func (d *DebugOutput) ChatErrorf(convID chat1.ConvIDStr, msg string, args ...any) {
	d.Errorf(msg, args...)
	if _, err := d.config.KBC.SendMessageByConvID(convID, "Something went wrong!"); err != nil && !IsIgnoredSendError(err) {
		d.Errorf("ChatErrorf: failed to send error message: %s", err)
	}
}

func (d *DebugOutput) ChatEcho(convID chat1.ConvIDStr, msg string, args ...any) {
	if err := d.SendMessageByConvIDOrFile(convID, "", msg, args...); err != nil && !IsIgnoredSendError(err) {
		d.Errorf("ChatEcho: failed to send echo message: %s", err)
	}
}

// SendMessageByConvIDOrFile sends a chat message, falling back to a file
// attachment when the message exceeds Keybase's maximum length.
func (d *DebugOutput) SendMessageByConvIDOrFile(convID chat1.ConvIDStr, title, msg string, args ...any) error {
	if d.config == nil || d.config.KBC == nil {
		return fmt.Errorf("chat not configured")
	}
	body := msg
	if len(args) > 0 {
		body = fmt.Sprintf(msg, args...)
	}
	if _, err := d.config.KBC.SendMessageByConvID(convID, "%s", body); err == nil ||
		IsIgnoredSendError(err) || !IsMessageTooLongError(err) {
		return err
	}

	filePath, err := writeTempFile(d.name, body)
	if err != nil {
		return err
	}
	GoWithRecover(d, func() {
		defer func() {
			time.Sleep(time.Minute)
			d.Debug("cleaning up %s", filePath)
			if err := os.Remove(filePath); err != nil {
				d.Errorf("unable to clean up %s: %v", filePath, err)
			}
		}()
		if _, err := d.config.KBC.SendAttachmentByConvID(convID, filePath, title); err != nil {
			d.Errorf("failed to send attachment %s: %s", filePath, err)
		}
	})
	return nil
}

func writeTempFile(namePrefix, content string) (string, error) {
	f, err := os.CreateTemp("", namePrefix+"-*.txt")
	if err != nil {
		return "", err
	}
	path := f.Name()
	defer func() {
		_ = f.Close()
	}()
	if _, err := f.WriteString(content); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Sync(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func (d *DebugOutput) Trace(err *error, format string, args ...any) func() {
	msg := fmt.Sprintf(format, args...)
	start := time.Now()
	fmt.Printf("+ %s: %s\n", d.name, msg)
	return func() {
		fmt.Printf("- %s: %s -> %s [time=%v]\n", d.name, msg, ErrToOK(err), time.Since(start))
	}
}

func IsIgnoredSendError(err error) bool {
	return IsDeletedConvError(err) || IsEphemeralKeyError(err)
}

func IsMessageTooLongError(err error) bool {
	// error created in https://github.com/keybase/client/blob/7d6aa64f3fba66adba7a5dd1cc7c523d5086a548/go/chat/msgchecker/plaintext_checker.go#L50
	return err != nil && strings.Contains(err.Error(), "exceeds the maximum length")
}

func IsDeletedConvError(err error) bool {
	// error created in https://github.com/keybase/client/blob/1985b18c4e7659bede1d4a2e68e4f68467acebc6/go/client/chat_svc_handler.go#L1407
	// error created in https://github.com/keybase/keybase/blob/9a82c96231ea2c6132532002e58bac80849265e6/go/chatbase/storage/sql_chat.go#L2324
	return strings.Contains(err.Error(), "no conversations matched") ||
		strings.Contains(err.Error(), "GetConvTriple called with unknown ConversationID")
}

func IsEphemeralKeyError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "unboxing teambotEK@generation:") ||
		strings.Contains(msg, "missing userEK@generation:") ||
		strings.Contains(msg, "Missing box for teambotEK@generation:")
}
