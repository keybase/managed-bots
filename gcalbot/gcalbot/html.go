package gcalbot

import (
	"bytes"
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
	body {
	  padding: 50px;
	  font-family: 'Lucida Sans', 'Lucida Sans Regular', 'Lucida Grande', 'Lucida Sans Unicode', Geneva,
		Verdana, sans-serif;
	  font-size: 22px;
	}
	a {
	  color: black;
	}

	.row {
	  display: flex;
	  flex-direction: row;
	}
	.column {
	  display: flex;
	  flex-direction: column;
	}
	.instructions {
	  margin-top: 16px;
	  text-align: center;
	}
	.quote {
	  font-family: 'Courier New', Courier, monospace;
	  background-color: bisque;
	  color: blue;
	  margin-left: 2px;
	  margin-right: 2px;
	  border-radius: 2px;
	}

	#divLogin {
	  justify-content: center;
	  align-items: center;
	  width: 600px;
	  margin: auto;
	}
	#divContainer {
	  justify-content: center;
	  align-items: center;
	}
	#imgLogo {
	  width: 300px;
	  height: 300px;
	}
  </style>
</head>
<body>`

const tmplFooter = `</body>
</html>`

type LoginPage struct {
	Title string
}

const tmplLogin = `{{template "header" .}}
  <div id="divContainer" class="column">
	<img src="/gcalbot/image/logo" id="imgLogo" />
	<div id="divLogin" class="column">
	  <span style="font-size: 32px; margin-bottom: 24px; text-align: center;">Login Required</span>
	  <span class="instructions">
		In order to configure your Google Calendar notifications, you must first login.
	  </span>
	  <span class="instructions">
		To start the login process, message <a target="_" href="https://keybase.io/gcalbot">@gcalbot</a> in the Keybase app with the command <span class="quote">!gcal configure</span>.
	  </span>
	</div>
{{template "footer" .}}`

type ConfigPage struct {
	Title         string
	KeybaseConvID chat1.ConvIDStr
	Account       string
	Accounts      []string
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
  <div id="divContainer" class="column">
	<form action="/gcalbot" method="post">
		<input type="hidden" name="conv_id" value="{{.KeybaseConvID}}">
		<input type="hidden" name="previous_account" value="{{.Account}}">
		<input type="hidden" name="previous_calendar" value="{{.CalendarID}}">

		<label for="account">Account:</label>
		<select name="account" onchange="this.form.submit()">
			<option value="" {{if .Account | not}} selected {{end}}>Select account</option>
			{{range .Accounts}}
			<option value="{{.}}" {{if eq . $.Account}} selected {{end}}>{{.}}</option>
			{{end}}
		</select>
		<br>

		<label for="calendar">Calendar:</label>
		<select name="calendar" {{if .Calendars | not}} disabled {{end}} onchange="this.form.submit()">
			<option value="" {{if .CalendarID | not}} selected {{end}}>Select calendar</option>
			{{range .Calendars}}
				<option value="{{.Id}}" {{if eq .Id $.CalendarID}} selected {{end}}>{{.Summary}}</option>
			{{end}}
		</select>
		<br>

		<label for="reminder">Reminders:</label>
		<select name="reminder" {{if .CalendarID | not}} disabled {{end}}>
			<option value="" {{if .CalendarID | not}} selected {{end}}>Do not send</option>
			{{range .Reminders}}
				<option value="{{.Minute}}" {{if eq .Minute $.Reminder}} selected {{end}}>{{.Summary}}</option>
			{{end}}
		</select>
		<br>

		<label for="invite">Invites:</label>
		<input type="checkbox" name="invite" {{if .CalendarID | not}} disabled {{end}} {{if .Invite}} checked {{end}}>
		<br>

		<input type="submit" value="Submit">
	</form>
  </div>
{{template "footer" .}}`

var templates = template.Must(template.Must(template.Must(template.Must(template.
	New("header").Parse(tmplHeader)).
	New("footer").Parse(tmplFooter)).
	New("login").Parse(tmplLogin)).
	New("config").Parse(tmplConfig))

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
