package triviabot

import (
	"encoding/json"
	"fmt"
	"html"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

type apiQuestion struct {
	Category         string
	Difficulty       string
	Question         string
	CorrectAnswer    string   `json:"correct_answer"`
	IncorrectAnswers []string `json:"incorrect_answers"`
}

type apiResponse struct {
	ReponseCode int
	Results     []apiQuestion
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

	kbc       *kbchat.API
	questions []question
	current   int
	convID    string
	curMsgID  chat1.MessageID
	answerCh  chan answer
}

func newSession(kbc *kbchat.API, convID string) *session {
	return &session{
		DebugOutput: base.NewDebugOutput("session", kbc),
		convID:      convID,
		answerCh:    make(chan answer),
		kbc:         kbc,
	}
}

func (s *session) getQuestions(total int) error {
	resp, err := http.Get(fmt.Sprintf("https://opentdb.com/api.php?amount=%d", total))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var apiResp apiResponse
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&apiResp); err != nil {
		return err
	}
	for _, r := range apiResp.Results {
		q := newQuestion(r)
		s.Debug("Question: %v correctAnswer: %d", q.answers, q.correctAnswer)
		s.questions = append(s.questions, newQuestion(r))
	}
	return nil
}

func (s *session) askQuestion() {
	q := s.questions[s.current]
	sendRes, err := s.kbc.SendMessageByConvID(s.convID, q.String())
	if err != nil {
		s.ChatDebug(s.convID, "failed to ask question: %s", err)
		return
	}
	if sendRes.Result.MessageID == nil {
		s.ChatDebug(s.convID, "failed to get message ID of question ask")
	}
	for index := range q.answers {
		if _, err := s.kbc.ReactByConvID(s.convID, *sendRes.Result.MessageID,
			base.NumberToEmoji(index+1)); err != nil {
			s.ChatDebug(s.convID, "failed to set reaction option: %s", err)
		}
	}
	s.curMsgID = *sendRes.Result.MessageID
	s.current++
}

func (s *session) waitForCorrectAnswer() {
	timeoutCh := make(chan struct{})
	doneCh := make(chan struct{})
	go func() {
		for {
			select {
			case answer := <-s.answerCh:
				if answer.msgID != s.curMsgID {
					s.Debug("ignoring answer for non-current question: cur: %d ans: %d", s.curMsgID,
						answer.msgID)
					continue
				}
				if answer.selection != s.questions[s.current].correctAnswer {
					s.ChatEcho(s.convID, "Incorrect answer of %s by %s", base.NumberToEmoji(answer.selection+1),
						answer.username)
				} else {
					s.ChatEcho(s.convID, "*Correct answer of %s by %s*", base.NumberToEmoji(answer.selection+1),
						answer.username)
					close(doneCh)
					return
				}
			case <-timeoutCh:
				return
			}
		}
	}()
	select {
	case <-time.After(30 * time.Second):
		s.ChatEcho(s.convID, "Times up, next question!")
		close(timeoutCh)
		return
	case <-doneCh:
	}
}

func (s *session) start(intotal int) error {
	total := defaultTotal
	if intotal > 0 {
		total = intotal
	}
	if err := s.getQuestions(total); err != nil {
		return err
	}
	go func() {
		for i := 0; i < len(s.questions); i++ {
			s.askQuestion()
			s.waitForCorrectAnswer()
		}
	}()
	return nil
}
