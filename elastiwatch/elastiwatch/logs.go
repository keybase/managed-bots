package elastiwatch

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"github.com/olivere/elastic"
)

type LogWatch struct {
	*base.DebugOutput
	db           *DB
	cli          *elastic.Client
	index, email string
	entries      []*entry
	emailer      base.Emailer
	sendCount    int
	lastSend     time.Time
	alertConvID  chat1.ConvIDStr
	emailConvID  chat1.ConvIDStr
	shutdownCh   chan struct{}
	peekCh       chan struct{}
}

func NewLogWatch(cli *elastic.Client, db *DB, index, email string, emailer base.Emailer,
	alertConvID, emailConvID chat1.ConvIDStr, debugConfig *base.ChatDebugOutputConfig) *LogWatch {
	return &LogWatch{
		DebugOutput: base.NewDebugOutput("LogWatch", debugConfig),
		cli:         cli,
		db:          db,
		index:       index,
		email:       email,
		emailer:     emailer,
		lastSend:    time.Now(),
		alertConvID: alertConvID,
		emailConvID: emailConvID,
		shutdownCh:  make(chan struct{}),
		peekCh:      make(chan struct{}),
	}
}

func (l *LogWatch) addAndCheckForSend(entries []*entry) {
	l.entries = append(l.entries, l.filterEntries(entries)...)
	threshold := 10000
	score := 0
	for _, e := range l.entries {
		switch e.Severity {
		case "INFO":
			score++
		case "WARNING":
			score += 5
		case "ERROR":
			score += 25
		case "CRITICAL":
			score += 10000
		}
	}
	if score > threshold {
		entriesCopy := make([]*entry, len(l.entries))
		copy(entriesCopy, l.entries)
		l.entries = nil
		l.Debug("threshold reached, sending: score: %d threshold: %d entries: %d",
			score, threshold, len(entriesCopy))
		go l.generateAndSend(entriesCopy)
	}
}

const backs = "```"

func (l *LogWatch) alertFromChunk(c chunk) {
	if c.Severity != "CRITICAL" {
		return
	}
	l.ChatEcho(l.alertConvID, `%d *CRITICAL* errors received
%s%s%s`, c.Count, backs, c.Message, backs)
}

func (l *LogWatch) alertEmail(subject string, chunks []chunk) {
	body := fmt.Sprintf("Email sent: %s", subject)
	for _, c := range chunks {
		if c.Severity == "INFO" {
			continue
		}
		body += fmt.Sprintf("\n%s %d %s", c.Severity, c.Count, c.Message)
	}
	l.ChatEcho(l.emailConvID, "```%s```", body)
}

func (l *LogWatch) filterEntries(entries []*entry) (res []*entry) {
	// get regexes
	deferrals, err := l.db.List()
	if err != nil {
		l.Errorf("failed to get filter list: %s", err)
		return entries
	}
	if len(deferrals) == 0 {
		return entries
	}
	var regexs []*regexp.Regexp
	for _, d := range deferrals {
		r, err := regexp.Compile(d.Regex)
		if err != nil {
			l.Errorf("invalid regex: %v err: %s", d, err)
			continue
		}
		regexs = append(regexs, r)
	}

	// filter
	isBlocked := func(msg string) bool {
		for _, r := range regexs {
			if r.Match([]byte(msg)) {
				l.Debug("filtering message: %s regex: %s", msg, r)
				return true
			}
		}
		return false
	}
	for _, e := range entries {
		if !isBlocked(e.Message) {
			res = append(res, e)
		}
	}
	return res
}

func (l *LogWatch) peek() {
	groupRes := newTreeifyGrouper(3).Group(l.entries)
	l.alertEmail("Peek results", groupRes)
}

func (l *LogWatch) generateAndSend(entries []*entry) {
	// do tree grouping
	groupRes := newTreeifyGrouper(3).Group(entries)
	indivRes := newTreeifyGrouper(0).Group(entries)

	for _, c := range groupRes {
		l.alertFromChunk(c)
	}
	var sections []renderSection
	sections = append(sections, renderSection{
		Heading: "Grouped Messages",
		Chunks:  groupRes,
	})
	sections = append(sections, renderSection{
		Heading: "Individual Messages",
		Chunks:  indivRes,
	})
	renderText, err := htmlRenderer{}.Render(sections)
	if err != nil {
		l.Debug("error rendering chunks: %s", err.Error())
	}

	dur := time.Since(l.lastSend).String()
	subject := fmt.Sprintf("Log Error Report - #%d - %s", l.sendCount, dur)
	l.alertEmail(subject, groupRes)
	if err := l.emailer.Send(l.email, subject, renderText); err != nil {
		l.Debug("error sending email: %s", err.Error())
	}
	l.sendCount++
	l.lastSend = time.Now()
}

func (l *LogWatch) runOnce() {
	query := elastic.NewBoolQuery().
		Must(elastic.NewRangeQuery("@timestamp").
			From(time.Now().Add(-time.Minute)).
			To(time.Now())).
		MustNot(elastic.NewTermQuery("severity", "debug"))
	res, err := l.cli.Search().
		Index(l.index).
		Query(query).
		Pretty(true).
		From(0).Size(10000).
		Do(context.Background())
	if err != nil {
		l.Debug("failed to run Elasticsearch query: %s", err)
		return
	}

	var entries []*entry
	if res.TotalHits() > 0 {
		l.Debug("query hits: %d", res.TotalHits())
		for _, hit := range res.Hits.Hits {
			entry, err := newEntry(*hit.Source)
			if err != nil {
				l.Errorf("failed to unmarshal log entry: %s", err)
				continue
			}
			entries = append(entries, entry)
		}
	} else {
		l.Debug("no query hits, doing nothing")
	}

	l.addAndCheckForSend(entries)
}

func (l *LogWatch) Run() error {
	l.Debug("log watch starting up...")
	if l.alertConvID != "" {
		l.Debug("alerting into convID: %s", l.alertConvID)
	}
	if l.emailConvID != "" {
		l.Debug("email notices into convID: %s", l.emailConvID)
	}
	l.runOnce()
	for {
		select {
		case <-l.shutdownCh:
			return nil
		case <-l.peekCh:
			l.peek()
		case <-time.After(time.Minute):
			l.runOnce()
		}
	}
}

func (l *LogWatch) Peek() {
	l.peekCh <- struct{}{}
}

func (l *LogWatch) Shutdown() (err error) {
	defer l.Trace(&err, "Shutdown")()
	close(l.shutdownCh)
	return nil
}
