package triviabot

import (
	"encoding/json"
	"fmt"
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
	CorrectAnswer    string
	IncorrectAnswers []string
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
	return question{
		category:      aq.Category,
		difficulty:    aq.Difficulty,
		question:      aq.Question,
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
		strAnswers = append(strAnswers, fmt.Sprintf("%s %s", base.NumberToEmoji(index), answer))
	}
	return res + strings.Join(strAnswers, "\n")
}

const defaultTotal = 10

type session struct {
	*base.DebugOutput

	kbc       *kbchat.API
	questions []question
	current   int
	convID    string
	curMsgID  chat1.MessageID
	answerCh  chan int
}

func newSession(kbc *kbchat.API, convID string) *session {
	return &session{
		DebugOutput: base.NewDebugOutput("session", kbc),
		convID:      convID,
		answerCh:    make(chan int),
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
		s.questions = append(s.questions, newQuestion(r))
	}
	return nil
}

func (s *session) askQuestion() {
	q := s.questions[s.current]
	s.ChatEcho(s.convID, q.String())
	s.current++
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
			select {
			case <-s.answerCh:
				s.ChatEcho(s.convID, "Answer given")
			case <-time.After(30 * time.Second):
				s.ChatEcho(s.convID, "Times up, next question!")
			}
		}
	}()
	return nil
}
