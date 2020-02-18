package gcalbot

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"

	"google.golang.org/api/calendar/v3"
)

const tmplHeader = `<!DOCTYPE html>
<html lang="en">
<head>
  <title>
	{{.Title}}
  </title>
  <style>
	body, select, button {
	  font-family: 'Lucida Sans', 'Lucida Sans Regular', 'Lucida Grande', 'Lucida Sans Unicode', Geneva,
		Verdana, sans-serif;
	  font-size: 22px;
	}
	body {
	  padding: 50px;
	}
	a {
	  color: black;
	}
	input[type=checkbox] {
	  height: 22px;
      width: 22px;
	}

	select {
		min-width: 248px;
		padding: 8px;
		white-space: nowrap;
		text-overflow: ellipsis;
	}
	.row {
	  display: flex;
	  flex-direction: row;
	  align-items: center;
	  margin-bottom: 12px;
	}

	.row label {
		margin-right: 12px;
	}

	.title {
		font-size: 48px;
		margin: 0;
		align-self: left;
	}
	.column {
	  display: flex;
	  flex-direction: column;
	}

	.column.left {
		align-items: flex-start;
	}
	.instructions {
	  margin-top: 16px;
	}
	.quote {
	  font-family: 'Courier New', Courier, monospace;
	  background-color: bisque;
	  color: blue;
	  margin-left: 2px;
	  margin-right: 2px;
	  border-radius: 2px;
	}

	.first-row {
		display: flex;
		flex-direction: row;
	}

	.row.account {
		margin-right: 12px;
	}

	.save-button {
		margin-top: 12px;
		width:  100px;
		height: 36px;
		font-size: 18px;
		background-color: #4c8eff;
		color: white;
		border: none;
		border-radius: 5px;
		cursor: pointer;
	}
	.save-button:disabled {
		opacity: 50%;
		cursor: not-allowed;
	}

	.conversation-title {
		font-size: 24px;
		margin-top: 12px;
		margin-bottom: 36px;
	}

	#divLogin {
	  justify-content: center;
	  align-items: center;
	  width: 600px;
	  margin: auto;
	}
	.container {
	  max-width: 750px;
	  margin: auto;
	  justify-content: center;
	  align-items: flex-start;
	}

	.logo-large {
	  width: 300px;
	  height: 300px;
	}

	.logo-small {
		width: 150px;
		height: 150px;
		margin-bottom: 24px;
	}
  </style>
  <meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body>`

const tmplFooter = `</body>
</html>`

type LoginPage struct {
	Title string
}

const tmplLogin = `{{template "header" .}}
  <div class="container column">
    <img src="/gcalbot/image/logo" class="logo-small" />
	<h1 class="title">
		Login Required
	</h1>
	<p class="instructions">
		In order to configure your Google Calendar notifications, you must first login.
	</p>
	<p class="instructions">
		To start the login process, message <a target="_" href="https://keybase.io/gcalbot">@gcalbot</a> in the Keybase
		app with the command <span class="quote">!gcal configure</span>.
	</p>
{{template "footer" .}}`

type ErrorPage struct {
	Title string
}

const tmplError = `{{template "header" .}}
  <div class="container column">
    <img src="/gcalbot/image/logo" class="logo-small" />
	<h1 class="title">
		An error occurred :(
	</h1>
	<p class="instructions">
		Please try again!
	</p>
	<p class="instructions">
		If the error is recurring, report the issue by messaging <a target="_" href="https://keybase.io/gcalbot">@gcalbot</a>
		in the Keybase app with the command <span class="quote">!gcalbot feedback</span> and some details around what went wrong.
	</p>
{{template "footer" .}}`

type AccountHelpPage struct {
	Title string
}

const tmplAccountHelp = `{{template "header" .}}
  <div class="container column">
    <img src="/gcalbot/image/logo" class="logo-small" />
	<h1 class="title">
		No connected Google accounts
	</h1>
	<p class="instructions">
		You haven't connected any Google accounts.
	</p>
	<p class="instructions">
		To connect an account, message <a target="_" href="https://keybase.io/gcalbot">@gcalbot</a> in the Keybase app with
		the command <span class="quote">!gcal accounts connect &lt;account nickname&gt;</span>.
	</p>
	<p class="instructions">
		For example, you can connect your work Google account using <span class="quote">!gcal accounts connect work</span>.
	</p>
  </div>
{{template "footer" .}}`

type ConfigPage struct {
	Title         string
	ConvID        chat1.ConvIDStr
	ConvHelpText  string
	ConvIsPrivate bool
	Account       string
	Accounts      []*Account
	CalendarID    string
	Calendars     []*calendar.CalendarListEntry
	Reminder      string
	Reminders     []ReminderType
	Invite        bool
}

type ReminderType struct {
	Minute  string
	Summary string
}

const tmplConfig = `{{template "header" .}}
  <div class="container column">
    <img src="/gcalbot/image/logo" class="logo-small" />
	<h1 class="title">
	  Configure Google Calendar
	</h1>
	<p class="conversation-title">
	  {{.ConvHelpText}}
	</p>
	<form action="/gcalbot" method="post" class="column">
		<input type="hidden" name="conv_id" value="{{.ConvID}}">
		<input type="hidden" name="previous_account" value="{{.Account}}">
		<input type="hidden" name="previous_calendar" value="{{.CalendarID}}">

		<div class="account row">
		<label for="account">Account:</label>
		<select name="account" onchange="this.form.submit()">
			<option value="" {{if .Account | not}} selected {{end}}>Select account</option>
			{{range .Accounts}}
			<option value="{{.AccountNickname}}" {{if eq .AccountNickname $.Account}} selected {{end}}>{{.AccountNickname}}</option>
			{{end}}
		</select>
		</div>

		<div class="row">
		<label for="calendar">Calendar:</label>
		<select name="calendar" {{if .Calendars | not}} disabled {{end}} onchange="this.form.submit()">
			<option value="" {{if .CalendarID | not}} selected {{end}}>Select calendar</option>
			{{range .Calendars}}
				<option value="{{.Id}}" {{if eq .Id $.CalendarID}} selected {{end}}>{{ellipsize .Summary 40}}</option>
			{{end}}
		</select>
		</div>

		{{if .CalendarID}}
		<div class="row">
		<label for="reminder">Send reminders for events... </label>
		<select name="reminder">
			<option value="" {{if .CalendarID | not}} selected {{end}}>Do not send</option>
			{{range .Reminders}}
				<option value="{{.Minute}}" {{if eq .Minute $.Reminder}} selected {{end}}>{{ellipsize .Summary 40}}</option>
			{{end}}
		</select>
		</div>

		{{if .ConvIsPrivate}}
		<div class="row">
		<label for="invite">Send notifications for event invites?</label>
		<input type="checkbox" name="invite" disabled {{if .Invite}} checked {{end}}>
		</div>
		{{end}}

		<input type="submit" value="Save" class="save-button"
			onclick="this.form.submit(); this.disabled=true; this.value='Saving...';">

		{{end}}

	</form>
  </div>
{{template "footer" .}}`

var templates = template.Must(template.Must(template.Must(template.Must(template.Must(template.Must(template.
	New("header").Parse(tmplHeader)).
	New("footer").Parse(tmplFooter)).
	New("login").Parse(tmplLogin)).
	New("error").Parse(tmplError)).
	New("account help").Parse(tmplAccountHelp)).
	New("config").Funcs(template.FuncMap{
	"ellipsize": func(input string, length int) string {
		runes := []rune(input)
		if len(runes) < length {
			return input
		}
		return fmt.Sprintf("%s...", string(runes[:(length-2)]))
	},
}).Parse(tmplConfig))

func (h *HTTPSrv) servePage(w http.ResponseWriter, name string, data interface{}) {
	var page bytes.Buffer
	if err := templates.ExecuteTemplate(&page, name, data); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.Errorf("error rendering page %s: %s", name, err)
		return
	}
	if _, err := io.Copy(w, &page); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.Errorf("error serving page %s: %s", name, err)
	}
}
