package processor

import (
	"github.com/mauv0809/ideal-tribble/internal/matchmaking"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/jobqueue"
)

// Processor handles the business logic of processing matches.
type Processor struct {
	store             Store
	jobQueue          jobqueue.JobQueue
	notifier          Notifier
	metrics           metrics.Metrics
	matchmakingService matchmaking.MatchmakingService
}
