package processor

import (
	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/slack"
)

// New creates a new Processor.
func New(store Store, notifier Notifier, metrics metrics.MetricsStore) *Processor {
	return &Processor{
		store:    store,
		notifier: notifier,
		metrics:  metrics,
	}
}

// ProcessMatches fetches matches that need processing and advances them through the state machine.
func (p *Processor) ProcessMatches(dryRun bool) {
	log.Info("Starting match processing...")
	matches, err := p.store.GetMatchesForProcessing()
	if err != nil {
		log.Error("Failed to get matches for processing", "error", err)
		return
	}

	if len(matches) == 0 {
		log.Info("No matches to process.")
		return
	}

	log.Info("Found matches to process", "count", len(matches))
	for _, match := range matches {
		p.processMatch(match, dryRun)
	}
	log.Info("Match processing finished.")
}

func (p *Processor) processMatch(match *playtomic.PadelMatch, dryRun bool) {
	log.Info("Processing match", "matchID", match.MatchID, "initial_status", match.ProcessingStatus, "game_status", match.GameStatus)

	for {
		currentState := match.ProcessingStatus
		log.Debug("Evaluating match state", "matchID", match.MatchID, "status", currentState)

		switch currentState {
		case playtomic.StatusNew:
			// If a match is already played, we never want to send a booking notification.
			if match.GameStatus == playtomic.GameStatusPlayed {
				// If results are also confirmed, we can jump straight to processing the result.
				if match.ResultsStatus == playtomic.ResultsStatusConfirmed {
					log.Info("Match is new but already played with confirmed results. Skipping booking notification and advancing to result available.", "matchID", match.MatchID)
					p.updateStatus(match, playtomic.StatusResultAvailable, dryRun)
				} else {
					// If played but results are not ready, just mark booking as "notified" to prevent future booking notifications.
					log.Info("Match is new and already played, but results are not confirmed. Skipping booking notification.", "matchID", match.MatchID)
					p.updateStatus(match, playtomic.StatusBookingNotified, dryRun)
				}
			} else {
				// This is a normal, upcoming match. Send the booking notification.
				log.Info("Match is new. Sending booking notification.", "matchID", match.MatchID)
				p.notifier.SendNotification(match, slack.BookingNotification, p.metrics, dryRun)
				p.updateStatus(match, playtomic.StatusBookingNotified, dryRun)
				p.metrics.Increment("matches_processed")
			}

		case playtomic.StatusBookingNotified:
			if match.GameStatus == playtomic.GameStatusPlayed && match.ResultsStatus == playtomic.ResultsStatusConfirmed {
				log.Info("Match has been played. Marking as result available.", "matchID", match.MatchID)
				p.updateStatus(match, playtomic.StatusResultAvailable, dryRun)
			}

		case playtomic.StatusResultAvailable:
			log.Info("Match result is available. Sending result notification.", "matchID", match.MatchID)
			p.notifier.SendNotification(match, slack.ResultNotification, p.metrics, dryRun)
			p.updateStatus(match, playtomic.StatusResultNotified, dryRun)

		case playtomic.StatusResultNotified:
			log.Info("Match result has been notified. Updating player stats.", "matchID", match.MatchID)
			if !dryRun {
				p.store.UpdatePlayerStats(match)
			}
			p.updateStatus(match, playtomic.StatusStatsUpdated, dryRun)

		case playtomic.StatusStatsUpdated:
			log.Info("Player stats updated. Marking match as complete.", "matchID", match.MatchID)
			p.updateStatus(match, playtomic.StatusCompleted, dryRun)

		case playtomic.StatusCompleted:
			log.Debug("Match is complete. No further processing needed.", "matchID", match.MatchID)
			return // End of the line for this match

		default:
			log.Warn("Unknown processing status", "status", currentState, "matchID", match.MatchID)
			return // Exit if status is unknown
		}

		// If the status hasn't changed, we're done with this match for now.
		if match.ProcessingStatus == currentState {
			log.Debug("Match state did not change. Finished processing for now.", "matchID", match.MatchID, "status", currentState)
			break
		}
	}
	log.Info("Finished processing match", "matchID", match.MatchID, "final_status", match.ProcessingStatus)
}

func (p *Processor) updateStatus(match *playtomic.PadelMatch, newStatus playtomic.ProcessingStatus, dryRun bool) {
	if dryRun {
		log.Info("[Dry Run] Would update match status", "matchID", match.MatchID, "from", match.ProcessingStatus, "to", newStatus)
		match.ProcessingStatus = newStatus // Update in-memory for the loop
		return
	}

	err := p.store.UpdateProcessingStatus(match.MatchID, newStatus)
	if err != nil {
		log.Error("Failed to update processing status", "error", err, "matchID", match.MatchID)
	} else {
		log.Debug("Successfully updated status", "matchID", match.MatchID, "from", match.ProcessingStatus, "to", newStatus)
		match.ProcessingStatus = newStatus // Keep the in-memory object in sync
	}
}
