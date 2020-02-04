package gcalbot

import (
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
)

type EventStatus string

const (
	EventStatusConfirmed EventStatus = "confirmed"
	EventStatusTentative EventStatus = "tentative"
	EventStatusCancelled EventStatus = "cancelled"
)

func FormatEvent(
	event *calendar.Event,
	accountNickname, calendarSummary string,
	timezone *time.Location,
	format24HourTime bool,
) (string, error) {
	message := `%s
> What: *%s*
> When: %s%s%s%s
> Calendar: %s%s`

	// strip protocol to skip unfurl prompt
	url := strings.TrimPrefix(event.HtmlLink, "https://")

	what := event.Summary

	// TODO(marcel): better date formatting for recurring events
	when, err := FormatTimeRange(event.Start, event.End, timezone, format24HourTime)
	if err != nil {
		return "", err
	}

	var where string
	if event.Location != "" {
		where = fmt.Sprintf("\n> Where: %s", event.Location)
	}

	var isOrganizer bool
	if event.Attendees == nil {
		isOrganizer = true
	} else {
		for _, attendee := range event.Attendees {
			if attendee.Self && attendee.Organizer {
				isOrganizer = true
			}
		}
	}

	var organizer string
	// don't show organizer for self-organized event
	if !isOrganizer {
		if event.Organizer.DisplayName != "" && event.Organizer.Email != "" {
			organizer = fmt.Sprintf("\n> Organizer: %s <%s>", event.Organizer.DisplayName, event.Organizer.Email)
		} else if event.Organizer.DisplayName != "" {
			organizer = fmt.Sprintf("\n> Organizer: %s", event.Organizer.DisplayName)
		} else if event.Organizer.Email != "" {
			organizer = fmt.Sprintf("\n> Organizer: %s", event.Organizer.Email)
		}
	}

	var conferenceData string
	if event.ConferenceData != nil {
		for _, entryPoint := range event.ConferenceData.EntryPoints {
			uri := strings.TrimPrefix(entryPoint.Uri, "https://")
			switch entryPoint.EntryPointType {
			case "video", "more":
				conferenceData += fmt.Sprintf("\n> Join online: %s", uri)
			case "phone":
				conferenceData += fmt.Sprintf("\n> Join by phone: %s", entryPoint.Label)
				if entryPoint.Pin != "" {
					conferenceData += fmt.Sprintf(" PIN: %s", entryPoint.Pin)
				}
			case "sip":
				conferenceData += fmt.Sprintf("\n> Join by SIP: %s", entryPoint.Label)
			}
		}
	}

	accountCalendar := fmt.Sprintf("%s [%s]", calendarSummary, accountNickname)

	// note: description can contain HTML
	var description string
	if event.Description != "" {
		// quote all newlines
		if strings.Contains(event.Description, "\n") {
			descriptionBody := strings.ReplaceAll(event.Description, "\n", "\n> > ")
			description = fmt.Sprintf("\n> Description:\n> > %s", descriptionBody)
		} else {
			description = fmt.Sprintf("\n> Description: %s", event.Description)
		}
	}

	return fmt.Sprintf(message,
		url, what, when, where, conferenceData, organizer, accountCalendar, description), nil
}
