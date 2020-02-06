package elastiwatch

import (
	"context"
	"fmt"
	"time"

	"github.com/keybase/managed-bots/base"
	"github.com/olivere/elastic"
)

type LogWatch struct {
	*base.DebugOutput
	cli          *elastic.Client
	index, email string
	entries      []*entry
	emailer      base.Emailer
	sendCount    int
	lastSend     time.Time
	shutdownCh   chan struct{}
}

func NewLogWatch(cli *elastic.Client, index, email string, emailer base.Emailer,
	debugConfig *base.ChatDebugOutputConfig) *LogWatch {
	return &LogWatch{
		DebugOutput: base.NewDebugOutput("LogWatch", debugConfig),
		cli:         cli,
		index:       index,
		email:       email,
		emailer:     emailer,
		lastSend:    time.Now(),
		shutdownCh:  make(chan struct{}),
	}
}

func (l *LogWatch) addAndCheckForSend(entries []*entry) {
	l.entries = append(l.entries, entries...)
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

func (l *LogWatch) generateAndSend(entries []*entry) {
	// do tree grouping
	groupRes := newTreeifyGrouper(3).Group(entries)
	indivRes := newTreeifyGrouper(0).Group(entries)

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
		l.Errorf("failed to run Elasticsearch query: %s", err)
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
	l.runOnce()
	for {
		select {
		case <-l.shutdownCh:
			return nil
		case <-time.After(time.Minute):
			l.runOnce()
		}
	}
}

func (l *LogWatch) Shutdown() error {
	close(l.shutdownCh)
	return nil
}
