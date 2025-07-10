package slack_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	internalslack "github.com/mauv0809/ideal-tribble/internal/slack"

	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockMetricsStore struct {
	counts map[string]int
}

func (m *mockMetricsStore) Increment(key string) {
	if m.counts == nil {
		m.counts = make(map[string]int)
	}
	m.counts[key]++
}
func (m *mockMetricsStore) GetAll() (map[string]int, error) {
	return m.counts, nil
}

func (m *mockMetricsStore) Get(key string) int {
	return m.counts[key]
}
func TestSlackClient_SendNotification_Booking(t *testing.T) {
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		assert.Equal(t, "C123", vals.Get("channel"))

		var blocks slack.Blocks
		err := json.Unmarshal([]byte(vals.Get("blocks")), &blocks)
		require.NoError(t, err)

		require.Len(t, blocks.BlockSet, 4)

		// A few basic checks to ensure we have the right formatter
		header := blocks.BlockSet[0].(*slack.HeaderBlock)
		assert.Contains(t, header.Text.Text, "New match booked!")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok": true, "ts": "12345.6789"}`))
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	api := slack.New("test-token", slack.OptionAPIURL(srv.URL+"/"))
	client := internalslack.NewClientWithAPI(api, "C123")
	metrics := &mockMetricsStore{}

	match := &playtomic.PadelMatch{
		ResourceName: "Court 1",
		Start:        time.Date(2024, 1, 1, 18, 0, 0, 0, time.UTC).Unix(),
		Teams: []playtomic.Team{
			{Players: []playtomic.Player{{Name: "Player A"}, {Name: "Player B"}}},
		},
		Price:           "100 SEK",
		BallBringerName: "Player A",
	}

	client.SendNotification(match, internalslack.BookingNotification, metrics, false)

	assert.True(t, handlerCalled, "Expected http handler to be called")
	assert.Equal(t, 1, metrics.Get("slack_notifications_sent"))
}

func TestSlackClient_SendNotification_Result(t *testing.T) {
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok": true, "ts": "12345.6789"}`))
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	api := slack.New("test-token", slack.OptionAPIURL(srv.URL+"/"))
	client := internalslack.NewClientWithAPI(api, "C123")
	metrics := &mockMetricsStore{}

	match := &playtomic.PadelMatch{} // Dummy match is enough for this test

	client.SendNotification(match, internalslack.ResultNotification, metrics, false)

	assert.True(t, handlerCalled, "Expected http handler to be called")
	assert.Equal(t, 1, metrics.Get("slack_notifications_sent"))
}

func TestSlackClient_SendNotification_DryRun(t *testing.T) {
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	api := slack.New("test-token", slack.OptionAPIURL(srv.URL+"/"))
	client := internalslack.NewClientWithAPI(api, "C123")
	metrics := &mockMetricsStore{}

	client.SendNotification(&playtomic.PadelMatch{}, internalslack.BookingNotification, metrics, true)

	assert.False(t, handlerCalled, "Expected http handler NOT to be called in dry run")
	assert.Equal(t, 0, metrics.Get("slack_notifications_sent"), "Metrics should not be incremented in dry run")
}
