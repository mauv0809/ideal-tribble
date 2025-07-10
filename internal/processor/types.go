package processor

import (
	"github.com/mauv0809/ideal-tribble/internal/metrics"
)

// Processor handles the business logic of processing matches.
type Processor struct {
	store    Store
	notifier Notifier
	metrics  metrics.MetricsStore
}
