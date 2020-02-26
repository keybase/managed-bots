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
	type eventItem struct {
		start   time.Time
		end     time.Time
		summary string
	}
	var eventItems []eventItem

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

		eventItems = append(eventItems, eventItem{
			start:   start,
			end:     end,
			summary: event.Summary,
		})
	}

	sort.Slice(eventItems, func(i, j int) bool {
		return eventItems[i].start.Before(eventItems[j].start)
	})

	formattedEvents := make([]string, len(eventItems))
	for index, item := range eventItems {
		startTime := FormatTime(item.start, format24HourTime, true)
		endTime := FormatTime(item.end, format24HourTime, true)
		formattedEvents[index] = fmt.Sprintf("> %s - %s *%s*",
			startTime, endTime, item.summary)
	}

	return strings.Join(formattedEvents, "\n"), nil
}
