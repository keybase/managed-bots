package gcalbot

import (
	"fmt"
	"sort"
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
	calendarSummary string,
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
		url, what, when, where, conferenceData, organizer, calendarSummary, description), nil
}

func FormatEventSchedule(
	events []*calendar.Event,
	timezone *time.Location,
	format24HourTime bool,
) (schedule string, err error) {
	// don't know the length of the array since we skip all-day events
	sort.Slice(events, func(i, j int) bool {
		startLeft, _, _, _ := ParseTime(events[i].Start, events[i].End)
		startRight, _, _, _ := ParseTime(events[j].Start, events[j].End)
		return startLeft.Before(startRight)
	})
	var formattedEvents []string
	for _, event := range events {
		start, end, isAllDay, err := ParseTime(event.Start, event.End)
		if err != nil {
			return "", err
		}
		if isAllDay {
			// TODO(marcel): support all day events
			continue
		}
		start = start.In(timezone)
		end = end.In(timezone)

		if format24HourTime {
			formattedEvents = append(formattedEvents, fmt.Sprintf("> %s - %s *%s*",
				start.Format("15:04"), end.Format("15:04"), event.Summary))
		} else {
			formattedEvents = append(formattedEvents, fmt.Sprintf("> %s - %s *%s*",
				start.Format("3:04pm"), end.Format("3:04pm"), event.Summary))
		}
	}
	return strings.Join(formattedEvents, "\n"), nil
}
