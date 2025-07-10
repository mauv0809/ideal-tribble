package http

import (
	"net/http"

	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/config"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/processor"
	"github.com/mauv0809/ideal-tribble/internal/slack"
)

type Server struct {
	Store           club.ClubStore
	Metrics         metrics.MetricsStore
	Cfg             config.Config
	PlaytomicClient playtomic.PlaytomicClient
	SlackClient     *slack.SlackClient
	Processor       *processor.Processor
	Router          *http.ServeMux
}
