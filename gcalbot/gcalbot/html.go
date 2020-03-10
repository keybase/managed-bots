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

	.title {
		font-size: 48px;
		margin: 0;
		align-self: left;
	}

	.row {
	  display: flex;
	  flex-direction: row;
	  align-items: center;
	  margin-top: 10px;
	}
	.column {
	  display: flex;
	  flex-direction: column;
	  margin-top: 10px;
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

	label {
		font-size: 13px;
		line-height: 17px;
		color: rgba(0, 0, 0, 0.5);
		font-weight: 600;
	}
	.select-label {
		margin-bottom: 8px;
	}
	.note {
	  display: flex;
	  flex-direction: row;
	  align-items: center;
	  font-size: 13px;
	  color: rgba(0, 0, 0, 0.5);
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
	.save-status {
		margin-top: 12px;
		margin-left: 12px;
		font-size: 18px;
    }
	#save-success {
		color: grey;
	}
	#save-error {
		color: red;
	}

	.conversation-title {
		font-size: 24px;
		margin-top: 12px;
		margin-bottom: 4px;
	}

	.daily-schedule {
		margin-left: 22px;
	}

	#divLogin {
	  justify-content: center;
	  align-items: center;
	  width: 600px;
	  margin: auto;
	}
	.container {
	  max-width: 850px;
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

	.select-container {
		position: relative;
		width: 256px;
	}
	.select-container select {
		appearance: none;
		-moz-appearance: none;
		-webkit-appearance: none;

		width: 100%;

		background-color: white;

		border-color: rgba(0, 0, 0, 0.1);
		border-radius: 4px;
		border-style: solid;
		border-width: 1px;

		padding-top: 4px;
		padding-bottom: 4px;
		padding-left: 16px;
		padding-right: 32px;

		font-size: 18px;
		white-space: nowrap;
		text-overflow: ellipsis;
	}
	.select-container:hover select {
		// blue
		border-color: #4C8EFF;
	}
	.select-container select:disabled {
		color: rgba(0, 0, 0, 0.247);
		border-color: rgba(0, 0, 0, 0.1);
		background-color: rgba(0, 0, 0, 0.025);
	}
	.select-container .caret {
		display: flex;
		position: absolute;

		top: 0;
		bottom: 0;
		right: 0;

		align-items: center;

		padding-left: 8px;
		padding-right: 16px;

		fill: rgba(0, 0, 0, 0.5);
	}
	.select-container:hover .caret {
		// blueDark
		fill: #3663EA;
	}
	.select-container select:disabled ~ .caret {
		fill: rgba(0, 0, 0, 0.247);
	}

	/* portrait smartphone */
	@media only screen and (max-width: 480px) {
	body {
		margin: 0;
		padding: 18px;
	}

	.column {
		width: 100%;
	}

	.logo-small {
		width: 100px;
		height: 100px;
		margin-bottom: 12px;
	}

	.title {
		font-size: 35px;
	}

	.conversation-title {
		margin-top: 4px;
	}

	label {
		font-size: 18px;
	}

	.select-container {
		margin-bottom: 12px;
		width: 100%;
	}

	.select-container select {
		font-size: 22px;
	}

	input[type="checkbox"] {
		width: 20px;
		height: 20px;
		margin: 4px 4px 0 0;
	}

	.daily-schedule {
		margin-left: 30px;
		margin-top: 8px;
	}

	.save-row {
		flex-direction: column;
	}

	.save-button {
		width: 100%;
		height: 40px;
		font-weight: 500;
	}

	.save-status {
		margin-left: 0;
	}
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
	Title    string
	CaretSVG template.HTML

	ConvID        chat1.ConvIDStr
	ConvHelpText  string
	ConvIsPrivate bool

	Account  string
	Accounts []*Account

	CalendarID string
	Calendars  []*calendar.CalendarListEntry

	Reminder        string
	ReminderOptions []ReminderType
	Invite          bool

	DSEnabled         bool
	DSDays            DaysToSendType
	DSDaysOptions     []DSDaysOption
	DSSchedule        ScheduleToSendType
	DSScheduleOptions []DSScheduleOption
	DSTime            string
	DSTimeOptions     [48]DSTimeOption

	Updated        bool
	PushNotAllowed bool
}

type ReminderType struct {
	Title  string
	Minute string
}

type DSDaysOption struct {
	Title string
	Days  DaysToSendType
}

type DSScheduleOption struct {
	Title    string
	Schedule ScheduleToSendType
}

type DSTimeOption struct {
	Title  string
	Minute string
}

const tmplConfig = `{{template "header" .}}
  <div class="container column">
    <img src="/gcalbot/image/logo" class="logo-small" />
	<h1 class="title">
	  Configure Google&nbsp;Calendar
	</h1>
	<p class="conversation-title">
	  {{.ConvHelpText}}
	</p>
	<form action="/gcalbot" method="post" class="column">
		<input type="hidden" name="conv_id" value="{{.ConvID}}">
		<input type="hidden" name="previous_account" value="{{.Account}}">
		<input type="hidden" name="previous_calendar" value="{{.CalendarID}}">

		<div class="column">
		<label for="account" class="select-label">Account</label>
		<div class="select-container">
			<select name="account" onchange="this.form.submit(); this.disabled=true;">
				<option value="" {{if .Account | not}} selected {{end}}>Select account</option>
				{{range .Accounts}}
				<option value="{{.AccountNickname}}" {{if eq .AccountNickname $.Account}} selected {{end}}>{{.AccountNickname}}</option>
				{{end}}
			</select>
			<div class="caret">{{.CaretSVG}}</div>
		</div>
		</div>

		<div class="column">
		<label for="calendar" class="select-label">Calendar</label>
		<div class="select-container">
			<select name="calendar" {{if .Calendars | not}} disabled {{end}} onchange="this.form.submit(); this.disabled=true;">
				{{if .Calendars | not}}<option value="">Select calendar</option>{{end}}
				{{range .Calendars}}
					<option value="{{.Id}}" {{if eq .Id $.CalendarID}} selected {{end}}>{{ellipsize .Summary 40}}</option>
				{{end}}
			</select>
			<div class="caret">{{.CaretSVG}}</div>
		</div>
		</div>

		{{if .CalendarID}}
		<div class="column">
		<label for="reminder" class="select-label">Send reminders for events... </label>
		<div class="select-container">
			<select name="reminder">
				<option value="" {{if .CalendarID | not}} selected {{end}}>Do not send</option>
				{{range .ReminderOptions}}
					<option value="{{.Minute}}" {{if eq .Minute $.Reminder}} selected {{end}}>{{ellipsize .Title 40}}</option>
				{{end}}
			</select>
			<div class="caret">{{.CaretSVG}}</div>
		</div>
		</div>

		{{if .ConvIsPrivate}}
		<div class="row">
		<label for="invite-input">
			<input type="checkbox" name="invite" id="invite-input" {{if .Invite}} checked {{end}}>
			Send notifications for event invites?
		</label>
		</div>
		{{end}}

		<div class="row">
		<label for="enable-daily">
			<input id="enable-daily" type="checkbox" name="ds_enabled" onclick="this.form.submit(); this.disabled=true;" {{if .DSEnabled}} checked {{end}}>
			Enable a daily schedule message?
		</label>
		</div>

		{{if .DSEnabled}}
		<div class="daily-schedule">
		<div class="column">
		<label for="ds_days" class="select-label">Daily schedule messages should be&nbsp;sent... </label>
		<div class="select-container">
			<select name="ds_days">
				{{range .DSDaysOptions}}
					<option value="{{.Days}}" {{if eq .Days $.DSDays}} selected {{end}}>{{.Title}}</option>
				{{end}}
			</select>
			<div class="caret">{{.CaretSVG}}</div>
		</div>
		</div>

		<div class="column">
		<label for="ds_schedule" class="select-label">Send reminders for&nbsp;events... </label>
		<div class="select-container">
			<select name="ds_schedule">
				{{range .DSScheduleOptions}}
					<option value="{{.Schedule}}" {{if eq .Schedule $.DSSchedule}} selected {{end}}>{{.Title}}</option>
				{{end}}
			</select>
			<div class="caret">{{.CaretSVG}}</div>
		</div>
		</div>

		<div class="column">
		<label for="ds_time" class="select-label">Daily schedule should be sent at... </label>
		<div class="select-container">
			<select name="ds_time">
				{{range .DSTimeOptions}}
					<option value="{{.Minute}}" {{if eq .Minute $.DSTime}} selected {{end}}>{{.Title}}</option>
				{{end}}
			</select>
			<div class="caret">{{.CaretSVG}}</div>
		</div>
		<div class="note">* Timezone pulled from your Google Calendar settings</div>
		</div>

		</div>
		{{end}}

		<div class="row save-row">
			<input type="submit" value="Save" class="save-button"
				onclick="this.form.submit(); this.disabled=true; this.value='Saving...';
						if(document.getElementById('save-success')) { document.getElementById('save-success').style.display='none' };">
			{{if .Updated}}<span id="save-success" class="save-status">Saved!</span>{{end}}
			{{if .PushNotAllowed}}<span id="save-error" class="save-status">Push notifications are not supported for this calendar</span>{{end}}
		</div>

		{{end}}

	</form>
  </div>
{{template "footer" .}}`

var caretSVG = template.HTML(`<svg xmlns="http://www.w3.org/2000/svg" width="8" height="8" viewBox="0 0 8 8"><path fill-rule="evenodd" d="M.64 2h6.72c.354 0 .64.265.64.592a.567.567 0 0 1-.162.394l-3.36 3.486a.677.677 0 0 1-.956 0L.162 2.986a.561.561 0 0 1 .052-.836A.67.67 0 0 1 .64 2z"/></svg>`)

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
