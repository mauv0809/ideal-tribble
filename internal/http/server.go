package http

import (
	"net/http"

	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/config"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/notifier"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/processor"
	"github.com/mauv0809/ideal-tribble/internal/pubsub"
)

func NewServer(store club.ClubStore, metricsSvc metrics.Metrics, metricsHandler http.Handler, cfg config.Config, playtomicClient playtomic.PlaytomicClient, notifier notifier.Notifier, processor *processor.Processor, pubsub pubsub.PubSubClient /*inngestClient inngest.InngestClient*/) *Server {
	server := &Server{
		Store:           store,
		Metrics:         metricsSvc,
		MetricsHandler:  metricsHandler,
		Cfg:             cfg,
		PlaytomicClient: playtomicClient,
		Notifier:        notifier,
		Processor:       processor,
		Router:          http.NewServeMux(),
		pubsub:          pubsub,
		//InngestClient:   inngestClient,
	}

	server.routes()
	return server
}

func (s *Server) routes() {
	// All handlers are wrapped with middleware using the Chain helper.
	// This makes it easy to add more middlewares in the future, like an authentication middleware.
	// e.g. Chain(s.MyHandler(), paramsMiddleware, authMiddleware)
	s.Router.Handle("/metrics", s.MetricsHandler)
	s.Router.Handle("/health", Chain(s.HealthCheckHandler(), paramsMiddleware))
	s.Router.Handle("/clear", Chain(s.ClearStoreHandler(), paramsMiddleware))
	s.Router.Handle("/members", Chain(s.ListMembersHandler(), paramsMiddleware))
	s.Router.Handle("/matches", Chain(s.ListMatchesHandler(), paramsMiddleware))
	s.Router.Handle("/fetch", Chain(s.FetchMatchesHandler(), paramsMiddleware))
	s.Router.Handle("/process", Chain(s.ProcessMatchesHandler(), paramsMiddleware))
	s.Router.Handle("/assign-ball-boy", Chain(s.BallBoyHandler(), paramsMiddleware))
	s.Router.Handle("/update-player-stats", Chain(s.UpdatePlayerStatsHandler(), paramsMiddleware))
	s.Router.Handle("/notify-booking", Chain(s.NotifyBookingHandler(), paramsMiddleware))
	s.Router.Handle("/notify-result", Chain(s.NotifyResultHandler(), paramsMiddleware))
	s.Router.Handle("/slack/command/leaderboard", Chain(s.LeaderboardCommandHandler(), paramsMiddleware))
	s.Router.Handle("/slack/command/player-stats", Chain(s.PlayerStatsCommandHandler(), paramsMiddleware))
	s.Router.Handle("/slack/command/level-leaderboard", Chain(s.LevelLeaderboardCommandHandler(), paramsMiddleware))
	//s.Router.Handle("/inngest/send", s.SendInngestEventHandler())
	//s.Router.Handle("/api/inngest", s.InngestClient.Serve())
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Router.ServeHTTP(w, r)
}
