package http

import (
	"net/http"

	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/config"
	"github.com/mauv0809/ideal-tribble/internal/jobqueue"
	"github.com/mauv0809/ideal-tribble/internal/matchmaking"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/notifier"
	"github.com/mauv0809/ideal-tribble/internal/pairings"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/processor"
)

type Server struct {
	Store              club.ClubStore
	Metrics            metrics.Metrics
	MetricsHandler     http.Handler
	Cfg                config.Config
	PlaytomicClient    playtomic.PlaytomicClient
	Notifier           notifier.Notifier
	Processor          *processor.Processor
	MatchmakingService matchmaking.MatchmakingService
	PairingsStore      pairings.PairingsStore
	Router             *http.ServeMux
	jobQueue           jobqueue.JobQueue
	//InngestClient   inngest.InngestClient
}
