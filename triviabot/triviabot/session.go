package triviabot

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/keybase1"
	"github.com/keybase/managed-bots/base"
)

var eligibleCategories = []int{9, 10, 11, 12, 14, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27}

type apiQuestion struct {
	Category         string
	Difficulty       string
	Question         string
	CorrectAnswer    string   `json:"correct_answer"`
	IncorrectAnswers []string `json:"incorrect_answers"`
}

type apiResponse struct {
	ResponseCode int `json:"response_code"`
	Results      []apiQuestion
}

type apiTokenResponse struct {
	ResponseCode int
	Token        string
}

type question struct {
	category      string
	difficulty    string
	question      string
	answers       []string
	correctAnswer int
}

func newQuestion(aq apiQuestion) question {
	a := append([]string{aq.CorrectAnswer}, aq.IncorrectAnswers...)
	rand.Shuffle(len(a), func(i, j int) { a[i], a[j] = a[j], a[i] })
	correctAnswer := 0
	for index, answer := range a {
		if answer == aq.CorrectAnswer {
			correctAnswer = index
			break
		}
	}
	for index := range a {
		a[index] = html.UnescapeString(a[index])
	}
	return question{
		category:      aq.Category,
		difficulty:    aq.Difficulty,
		question:      html.UnescapeString(aq.Question),
		answers:       a,
		correctAnswer: correctAnswer,
	}
}

func (q question) Answer() string {
	return q.answers[q.correctAnswer]
}

func (q question) String() (res string) {
	res = fmt.Sprintf(`*Question:* %s
Difficulty: %s
Category: %s
`, q.question, q.difficulty, q.category)
	var strAnswers []string
	for index, answer := range q.answers {
		strAnswers = append(strAnswers, fmt.Sprintf("%s %s", base.NumberToEmoji(index+1), answer))
	}
	return res + strings.Join(strAnswers, "\n")
}

const defaultTotal = 10

type answer struct {
	selection int
	msgID     chat1.MessageID
	username  string
}

type session struct {
	*base.DebugOutput

	kbc            *kbchat.API
	db             *DB
	convID         chat1.ConvIDStr
	numUsersInConv int
	curQuestion    *question
	curMsgID       chat1.MessageID
	answerCh       chan answer
	stopCh         chan struct{}
	dupCheck       map[string]bool
}

func newSession(kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig, db *DB, convID chat1.ConvIDStr) *session {
	return &session{
		DebugOutput: base.NewDebugOutput("session", debugConfig),
		db:          db,
		convID:      convID,
		answerCh:    make(chan answer, 10),
		kbc:         kbc,
		stopCh:      make(chan struct{}),
		dupCheck:    make(map[string]bool),
	}
}

func (s *session) getCategory() int {
	return eligibleCategories[rand.Intn(len(eligibleCategories))]
}

func (s *session) getAPIToken() (string, error) {
	resp, err := http.Get("https://opentdb.com/api_token.php?command=request")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var apiResp apiTokenResponse
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&apiResp); err != nil {
		return "", err
	}
	if apiResp.ResponseCode != 0 {
		return "", fmt.Errorf("error from token API: %d", apiResp.ResponseCode)
	}
	return apiResp.Token, nil
}

var errForceAPI = errors.New("API token fetch requested")

func (s *session) getToken(forceAPI bool) (token string, err error) {
	if !forceAPI {
		token, err = s.db.GetAPIToken(s.convID)
	} else {
		err = errForceAPI
	}
	if err != nil {
		s.Debug("getToken: failed to get token from DB: %s", err)
		if token, err = s.getAPIToken(); err != nil {
			s.ChatErrorf(s.convID, "getToken: failed to get token from API: %s", err)
			return "", err
		}
		if err := s.db.SetAPIToken(s.convID, token); err != nil {
			s.Errorf("getToken: failed to set token in DB: %s", err)
		}
	} else {
		s.Debug("getToken: DB hit")
	}
	return token, nil
}

var errTokenExpired = errors.New("token expired")

func (s *session) getNextQuestion() error {
	token, err := s.getToken(false)
	if err != nil {
		s.Errorf("getNextQuestion: failed to get token: %s", err)
		return err
	}
	var apiResp apiResponse
	getQuestion := func(token string) error {
		url := fmt.Sprintf("https://opentdb.com/api.php?amount=1&category=%d&token=%s&type=multiple",
			s.getCategory(), token)
		s.Debug("getNextQuestion: url: %s", url)
		resp, err := http.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(&apiResp); err != nil {
			return err
		}
		if apiResp.ResponseCode == 3 {
			// need new token
			return errTokenExpired
		}
		return nil
	}
	if err := getQuestion(token); err != nil {
		if err == errTokenExpired {
			s.Debug("getNextQuestion: token expired, trying again")
			if token, err = s.getToken(true); err != nil {
				s.Errorf("getNextQuestion: failed to get token: %s", err)
				return err
			}
			if err := getQuestion(token); err != nil {
				s.Errorf("getNextQuestion: failed to get next question after token error: %s", err)
				return err
			}
		}
	}
	if len(apiResp.Results) > 0 {
		q := newQuestion(apiResp.Results[0])
		s.curQuestion = &q
		res, err := s.kbc.ListMembersByConvID(s.convID)
		if err != nil {
			return err
		}
		// ignore bot users here
		s.numUsersInConv = len(res.Owners) + len(res.Admins) + len(res.Writers) + len(res.Readers)
		// If we're not a bot member exclude us from the member count.
		if !s.isBotRole(res) {
			s.numUsersInConv--
		}
	}
	return nil
}

func (s *session) isBotRole(members keybase1.TeamMembersDetails) bool {
	for _, member := range append(members.Bots, members.RestrictedBots...) {
		if member.Username == s.kbc.GetUsername() {
			return true
		}
	}
	return false
}

func (s *session) askQuestion() error {
	if s.curQuestion == nil {
		s.Debug("askQuestion: current question nil, bailing")
		return errors.New("no question to ask")
	}
	q := *s.curQuestion
	s.Debug("askQuestion: question: %s answer: %d", q.question, q.correctAnswer+1)
	sendRes, err := s.kbc.SendMessageByConvID(s.convID, q.String())
	if err != nil {
		s.ChatErrorf(s.convID, "askQuestion: failed to ask question: %s", err)
		return err
	}
	if sendRes.Result.MessageID == nil {
		s.ChatErrorf(s.convID, "askQuestion: failed to get message ID of question ask")
	}
	for index := range q.answers {
		if _, err := s.kbc.ReactByConvID(s.convID, *sendRes.Result.MessageID,
			base.NumberToEmoji(index+1)); err != nil {
			s.ChatErrorf(s.convID, "askQuestion: failed to set reaction option: %s", err)
		}
	}
	s.curMsgID = *sendRes.Result.MessageID
	return nil
}

func (s *session) getAnswerPoints(a answer, q question) (isCorrect bool, pointAdjust int) {
	if a.selection != q.correctAnswer {
		return false, -5
	}
	switch q.difficulty {
	case "easy":
		return true, 5
	case "medium":
		return true, 10
	case "hard":
		return true, 15
	default:
		s.Debug("getAnswerPoints: unknown difficulty: %s", q.difficulty)
		return true, 5
	}
}

func (s *session) dupKey(username string) string {
	return fmt.Sprintf("%s:%d", username, s.curMsgID)
}

func (s *session) checkDupe(username string) bool {
	return s.dupCheck[s.dupKey(username)]
}

func (s *session) regDupe(username string) {
	s.dupCheck[s.dupKey(username)] = true
}

func (s *session) numAnswers() int {
	return len(s.dupCheck)
}

func (s *session) waitForCorrectAnswer() {
	timeoutCh := make(chan struct{})
	doneCh := make(chan struct{})
	base.GoWithRecover(s.DebugOutput, func() {
		for {
			select {
			case <-s.stopCh:
				return
			case <-timeoutCh:
				return
			case answer := <-s.answerCh:
				if s.checkDupe(answer.username) {
					s.Debug("ignoring duplicate answer from: %s", answer.username)
					continue
				}
				if answer.msgID != s.curMsgID {
					s.Debug("ignoring answer for non-current question: cur: %d ans: %d", s.curMsgID,
						answer.msgID)
					continue
				}
				if s.curQuestion == nil {
					s.Debug("ignoring answer since curQuestion is nil")
					continue
				}
				isCorrect, pointAdjust := s.getAnswerPoints(answer, *s.curQuestion)
				if err := s.db.RecordAnswer(s.convID, answer.username, pointAdjust, isCorrect); err != nil {
					s.Errorf("waitForCorrectAnswer: failed to record answer: %s", err)
				}
				s.regDupe(answer.username)
				if !isCorrect {
					s.ChatEcho(s.convID, "Incorrect answer of %s by %s (%d points)",
						base.NumberToEmoji(answer.selection+1), answer.username, pointAdjust)
					// If no one else can answer short circuit instead of forcing the timeout
					if s.numAnswers() >= s.numUsersInConv {
						s.ChatEcho(s.convID, "Next question!\nCorrect answer was %s *%q*",
							base.NumberToEmoji(s.curQuestion.correctAnswer+1), s.curQuestion.Answer())
						s.curQuestion = nil
						close(doneCh)
						return
					}
				} else {
					s.ChatEcho(s.convID, "*Correct answer of %s by %s (%d points)*",
						base.NumberToEmoji(answer.selection+1), answer.username, pointAdjust)
					s.curQuestion = nil
					close(doneCh)
					return
				}

			}
		}
	})
	select {
	case <-time.After(20 * time.Second):
		s.ChatEcho(s.convID, "Times up, next question!\nCorrect answer was %s *%q*",
			base.NumberToEmoji(s.curQuestion.correctAnswer+1), s.curQuestion.Answer())
		close(timeoutCh)
		return
	case <-doneCh:
	case <-s.stopCh:
	}
}

func (s *session) start(intotal int) (doneCb chan struct{}, err error) {
	doneCb = make(chan struct{})
	total := defaultTotal
	if intotal > 0 {
		total = intotal
	}
	base.GoWithRecover(s.DebugOutput, func() {
		defer close(doneCb)
		for i := 0; i < total; i++ {
			select {
			case <-s.stopCh:
				return
			default:
			}
			if err := s.getNextQuestion(); err != nil {
				s.ChatErrorf(s.convID, "start: failed to get next question: %s", err)
				continue
			}
			if err := s.askQuestion(); err != nil {
				s.ChatErrorf(s.convID, "start: failed to ask question: %s", err)
				continue
			}
			s.waitForCorrectAnswer()
		}
	})
	return doneCb, nil
}

func (s *session) stop() {
	close(s.stopCh)
}
