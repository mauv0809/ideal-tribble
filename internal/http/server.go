package http

import (
	"net/http"

	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/config"
	"github.com/mauv0809/ideal-tribble/internal/http/handlers"
	"github.com/mauv0809/ideal-tribble/internal/matchmaking"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/notifier"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/processor"
	"github.com/mauv0809/ideal-tribble/internal/pubsub"
)

func NewServer(store club.ClubStore, metricsSvc metrics.Metrics, metricsHandler http.Handler, cfg config.Config, playtomicClient playtomic.PlaytomicClient, notifier notifier.Notifier, processor *processor.Processor, matchmakingService matchmaking.MatchmakingService, pubsub pubsub.PubSubClient /*inngestClient inngest.InngestClient*/) *Server {
	server := &Server{
		Store:              store,
		Metrics:            metricsSvc,
		MetricsHandler:     metricsHandler,
		Cfg:                cfg,
		PlaytomicClient:    playtomicClient,
		Notifier:           notifier,
		Processor:          processor,
		MatchmakingService: matchmakingService,
		Router:             http.NewServeMux(),
		pubsub:             pubsub,
		//InngestClient:   inngestClient,
	}

	server.routes()
	return server
}

func (s *Server) routes() {
	// All handlers are wrapped with middleware using the Chain helper.
	// This makes it easy to add more middlewares in the future, like an authentication middleware.
	// e.g. Chain(s.MyHandler(), paramsMiddleware, authMiddleware)

	// Metrics (no middleware needed)
	s.Router.Handle("/metrics", s.MetricsHandler)

	// Health and operational endpoints
	s.Router.Handle("/health", Chain(handlers.HealthCheckHandler(s.Store), paramsMiddleware))
	s.Router.Handle("/clear", Chain(handlers.ClearStoreHandler(s.Store), paramsMiddleware))

	// REST API endpoints
	s.Router.Handle("/members", Chain(handlers.ListMembersHandler(s.Store), paramsMiddleware))
	s.Router.Handle("/matches", Chain(handlers.ListMatchesHandler(s.Store), paramsMiddleware))
	s.Router.Handle("/leaderboard", Chain(handlers.LeaderboardHandler(s.Store), paramsMiddleware))

	// Scheduled endpoints (Cloud Scheduler)
	s.Router.Handle("/fetch", Chain(handlers.FetchMatchesHandler(s.Store, s.Metrics, s.Cfg, s.PlaytomicClient), paramsMiddleware))
	s.Router.Handle("/process", Chain(handlers.ProcessMatchesHandler(s.Processor), paramsMiddleware))

	// Pub/Sub endpoints
	s.Router.Handle("/assign-ball-boy", Chain(handlers.BallBoyHandler(s.Processor, s.pubsub), paramsMiddleware))
	s.Router.Handle("/update-player-stats", Chain(handlers.UpdatePlayerStatsHandler(s.Processor, s.pubsub), paramsMiddleware))
	s.Router.Handle("/update-weekly-stats", Chain(handlers.UpdateWeeklyStatsHandler(s.Processor, s.pubsub), paramsMiddleware))
	s.Router.Handle("/notify-booking", Chain(handlers.NotifyBookingHandler(s.Processor, s.pubsub), paramsMiddleware))
	s.Router.Handle("/notify-result", Chain(handlers.NotifyResultHandler(s.Processor, s.pubsub), paramsMiddleware))

	// Slack command endpoints
	s.Router.Handle("/slack/command/leaderboard", Chain(handlers.LeaderboardCommandHandler(s.Store, s.Notifier), s.VerifySlackSignature, paramsMiddleware))
	s.Router.Handle("/slack/command/player-stats", Chain(handlers.PlayerStatsCommandHandler(s.Store, s.Notifier), s.VerifySlackSignature, paramsMiddleware))
	s.Router.Handle("/slack/command/level-leaderboard", Chain(handlers.LevelLeaderboardCommandHandler(s.Store, s.Notifier), s.VerifySlackSignature, paramsMiddleware))
	s.Router.Handle("/slack/command/match", Chain(handlers.MatchCommandHandler(s.Store, s.Notifier, s.MatchmakingService), s.VerifySlackSignature, paramsMiddleware))

	// Slack events endpoint
	s.Router.Handle("/slack/events", Chain(handlers.SlackEventsHandler(s.Store, s.Notifier, s.MatchmakingService, s.Cfg), s.VerifySlackSignature, paramsMiddleware))

	// Test endpoints (for development)
	s.Router.Handle("/test/react", Chain(handlers.TestReactHandler(s.Store, s.MatchmakingService, s.Notifier), paramsMiddleware))

	//s.Router.Handle("/inngest/send", s.SendInngestEventHandler())
	//s.Router.Handle("/api/inngest", s.InngestClient.Serve())
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Router.ServeHTTP(w, r)
}
