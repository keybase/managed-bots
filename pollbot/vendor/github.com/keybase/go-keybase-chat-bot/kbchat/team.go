package kbchat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/keybase1"
)

type ListTeamMembers struct {
	Result keybase1.TeamDetails `json:"result"`
	Error  Error                `json:"error"`
}

type ListMembersOutputMembersCategory struct {
	Username string `json:"username"`
	FullName string `json:"fullName"`
}

type ListUserMemberships struct {
	Result keybase1.AnnotatedTeamList `json:"result"`
	Error  Error                      `json:"error"`
}

func (a *API) ListMembersOfTeam(teamName string) (keybase1.TeamMembersDetails, error) {
	empty := keybase1.TeamMembersDetails{}

	apiInput := fmt.Sprintf(`{"method": "list-team-memberships", "params": {"options": {"team": "%s"}}}`, teamName)
	cmd := a.runOpts.Command("team", "api")
	cmd.Stdin = strings.NewReader(apiInput)
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		return empty, fmt.Errorf("failed to call keybase team api: %v", err)
	}

	members := ListTeamMembers{}
	err = json.Unmarshal(bytes, &members)
	if err != nil {
		return empty, fmt.Errorf("failed to parse output from keybase team api: %v", err)
	}
	if members.Error.Message != "" {
		return empty, fmt.Errorf("received error from keybase team api: %s", members.Error.Message)
	}
	return members.Result.Members, nil
}

func (a *API) ListUserMemberships(username string) ([]keybase1.AnnotatedMemberInfo, error) {
	empty := []keybase1.AnnotatedMemberInfo{}

	apiInput := fmt.Sprintf(`{"method": "list-user-memberships", "params": {"options": {"username": "%s"}}}`, username)
	cmd := a.runOpts.Command("team", "api")
	cmd.Stdin = strings.NewReader(apiInput)
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		return empty, fmt.Errorf("failed to call keybase team api: %v", err)
	}

	members := ListUserMemberships{}
	err = json.Unmarshal(bytes, &members)
	if err != nil {
		return empty, fmt.Errorf("failed to parse output from keybase team api: %v", err)
	}
	if members.Error.Message != "" {
		return empty, fmt.Errorf("received error from keybase team api: %s", members.Error.Message)
	}
	return members.Result.Teams, nil
}
