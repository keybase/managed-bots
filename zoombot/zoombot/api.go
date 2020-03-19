package zoombot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	apiBaseURL       = "https://api.zoom.us"
	apiBaseURLV2     = "https://api.zoom.us/v2"
	currentUserID    = "me"
	invalidTokenCode = 124
)

// Common

type MeetingType int

const (
	InstantMeeting              MeetingType = 1
	ScheduledMeeting            MeetingType = 2
	RecurringMeetingNoFixedTime MeetingType = 3
	RecurringMeetingFixedTime   MeetingType = 8
)

type TrackingField struct {
	Field string `json:"field,omitempty"`
	Value string `json:"value,omitempty"`
}

type Recurrence struct {
	Type           int    `json:"type,omitempty"`
	RepeatInterval int    `json:"repeat_interval,omitempty"`
	WeeklyDays     string `json:"weekly_days,omitempty"`
	MonthlyDay     int    `json:"monthly_day,omitempty"`
	MonthlyWeek    int    `json:"monthly_week,omitempty"`
	MonthlyWeekDay int    `json:"monthly_week_day,omitempty"`
	EndTimes       int    `json:"end_times,omitempty"`
	EndDateTime    string `json:"end_date_time,omitempty"`
}

type GlobalDialInNumbers struct {
	City        string `json:"city"`
	Country     string `json:"country"`
	CountryName string `json:"country_name"`
	Number      string `json:"number"`
	Type        string `json:"type"`
}

// Get User

type GetUserResponse struct {
	ID                 string        `json:"id"`
	FirstName          string        `json:"first_name"`
	LastName           string        `json:"last_name"`
	Email              string        `json:"email"`
	Type               int           `json:"type"`
	RoleName           string        `json:"role_name"`
	Pmi                int           `json:"pmi"`
	UsePmi             bool          `json:"use_pmi"`
	PersonalMeetingURL string        `json:"personal_meeting_url"`
	Timezone           string        `json:"timezone"`
	Verified           int           `json:"verified"`
	Dept               string        `json:"dept"`
	CreatedAt          time.Time     `json:"created_at"`
	LastLoginTime      time.Time     `json:"last_login_time"`
	LastClientVersion  string        `json:"last_client_version"`
	PicURL             string        `json:"pic_url"`
	HostKey            string        `json:"host_key"`
	Jid                string        `json:"jid"`
	GroupIds           []interface{} `json:"group_ids"`
	ImGroupIds         []string      `json:"im_group_ids"`
	AccountID          string        `json:"account_id"`
	Language           string        `json:"language"`
	PhoneCountry       string        `json:"phone_country"`
	PhoneNumber        string        `json:"phone_number"`
	Status             string        `json:"status"`
}

// Create Meeting

type CreateMeetingRequest struct {
	Topic     string      `json:"topic,omitempty"`
	Type      MeetingType `json:"type,omitempty"`
	StartTime string      `json:"start_time,omitempty"`
	Duration  int         `json:"duration,omitempty"`
	Timezone  string      `json:"timezone,omitempty"`
	Password  string      `json:"password,omitempty"`
	Agenda    string      `json:"agenda,omitempty"`

	TrackingFields []TrackingField               `json:"tracking_fields,omitempty"`
	Recurrence     *Recurrence                   `json:"recurrence,omitempty"`
	Settings       *CreateMeetingRequestSettings `json:"settings,omitempty"`
}

type CreateMeetingRequestSettings struct {
	HostVideo                    bool     `json:"host_video,omitempty"`
	ParticipantVideo             bool     `json:"participant_video,omitempty"`
	CnMeeting                    bool     `json:"cn_meeting,omitempty"`
	InMeeting                    bool     `json:"in_meeting,omitempty"`
	JoinBeforeHost               bool     `json:"join_before_host,omitempty"`
	MuteUponEntry                bool     `json:"mute_upon_entry,omitempty"`
	Watermark                    bool     `json:"watermark,omitempty"`
	UsePmi                       bool     `json:"use_pmi,omitempty"`
	ApprovalType                 int      `json:"approval_type,omitempty"`
	RegistrationType             int      `json:"registration_type,omitempty"`
	Audio                        string   `json:"audio,omitempty"`
	AutoRecording                string   `json:"auto_recording,omitempty"`
	EnforceLogin                 bool     `json:"enforce_login,omitempty"`
	EnforceLoginDomains          string   `json:"enforce_login_domains,omitempty"`
	AlternativeHosts             string   `json:"alternative_hosts,omitempty"`
	GlobalDialInCountries        []string `json:"global_dial_in_countries,omitempty"`
	RegistrantsEmailNotification bool     `json:"registrants_email_notification,omitempty"`
}

type CreateMeetingResponse struct {
	CreatedAt time.Time   `json:"created_at"`
	Duration  int         `json:"duration"`
	HostID    string      `json:"host_id"`
	ID        int         `json:"id"`
	JoinURL   string      `json:"join_url"`
	StartTime time.Time   `json:"start_time"`
	StartURL  string      `json:"start_url"`
	Status    string      `json:"status"`
	Timezone  string      `json:"timezone"`
	Topic     string      `json:"topic"`
	Type      MeetingType `json:"type"`
	UUID      string      `json:"uuid"`

	Settings CreateMeetingResponseSettings `json:"settings"`
}

type CreateMeetingResponseSettings struct {
	AlternativeHosts             string                `json:"alternative_hosts"`
	ApprovalType                 int                   `json:"approval_type"`
	Audio                        string                `json:"audio"`
	AutoRecording                string                `json:"auto_recording"`
	CloseRegistration            bool                  `json:"close_registration"`
	CnMeeting                    bool                  `json:"cn_meeting"`
	EnforceLogin                 bool                  `json:"enforce_login"`
	EnforceLoginDomains          string                `json:"enforce_login_domains"`
	GlobalDialInCountries        []string              `json:"global_dial_in_countries"`
	GlobalDialInNumbers          []GlobalDialInNumbers `json:"global_dial_in_numbers"`
	HostVideo                    bool                  `json:"host_video"`
	InMeeting                    bool                  `json:"in_meeting"`
	JoinBeforeHost               bool                  `json:"join_before_host"`
	MuteUponEntry                bool                  `json:"mute_upon_entry"`
	ParticipantVideo             bool                  `json:"participant_video"`
	RegistrantsConfirmationEmail bool                  `json:"registrants_confirmation_email"`
	UsePmi                       bool                  `json:"use_pmi"`
	WaitingRoom                  bool                  `json:"waiting_room"`
	Watermark                    bool                  `json:"watermark"`
	RegistrantsEmailNotification bool                  `json:"registrants_email_notification"`
}

// Deauthorization

type DeauthorizationRequest struct {
	Event   string               `json:"event"`
	Payload DeauthorizationEvent `json:"payload"`
}

type DeauthorizationEvent struct {
	UserDataRetention   string `json:"user_data_retention"`
	AccountID           string `json:"account_id"`
	UserID              string `json:"user_id"`
	Signature           string `json:"signature"`
	DeauthorizationTime string `json:"deauthorization_time"`
	ClientID            string `json:"client_id"`
}

// Compliance

type DataComplianceRequest struct {
	ClientID                     string               `json:"client_id"`
	UserID                       string               `json:"user_id"`
	AccountID                    string               `json:"account_id"`
	DeauthorizationEventReceived DeauthorizationEvent `json:"deauthorization_event_received"`
	ComplianceCompleted          bool                 `json:"compliance_completed"`
}

type DataComplianceResponse struct {
	UserDataRetention   bool   `json:"user_data_retention"`
	AccountID           string `json:"account_id"`
	UserID              string `json:"user_id"`
	Signature           string `json:"signature"`
	DeauthorizationTime string `json:"deauthorization_time"`
	ClientID            string `json:"client_id"`
}

func GetUser(client *http.Client, userID string) (*GetUserResponse, error) {
	apiURL := fmt.Sprintf("%s/users/%s", apiBaseURLV2, userID)
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp.StatusCode, data)
	}

	var user GetUserResponse
	err = json.Unmarshal(data, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func CreateMeeting(client *http.Client, userID string, request *CreateMeetingRequest) (*CreateMeetingResponse, error) {
	apiURL := fmt.Sprintf("%s/users/%s/meetings", apiBaseURLV2, userID)
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, parseError(resp.StatusCode, data)
	}

	var meeting CreateMeetingResponse
	err = json.Unmarshal(data, &meeting)
	if err != nil {
		return nil, err
	}

	return &meeting, nil
}

func DataCompliance(clientID, clientSecret string, request *DataComplianceRequest) (*DataComplianceResponse, error) {
	apiURL := fmt.Sprintf("%s/oauth/data/compliance", apiBaseURL)
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Add("content-type", "application/json")
	req.SetBasicAuth(clientID, clientSecret)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp.StatusCode, data)
	}

	var event DataComplianceResponse
	err = json.Unmarshal(data, &event)
	if err != nil {
		return nil, err
	}

	return &event, nil
}

type ZoomAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e ZoomAPIError) Error() string {
	return e.Message
}

func parseError(statusCode int, data []byte) error {
	var errorResponse ZoomAPIError
	err := json.Unmarshal(data, &errorResponse)
	if err != nil || errorResponse.Code == 0 {
		return fmt.Errorf("statusCode: %d, error: %s", statusCode, data)
	}
	return errorResponse
}
