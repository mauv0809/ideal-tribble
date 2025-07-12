package processor

import (
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/pubsub"
)

// Processor handles the business logic of processing matches.
type Processor struct {
	store    Store
	pubsub   pubsub.PubSubClient
	notifier Notifier
	metrics  metrics.Metrics
}
