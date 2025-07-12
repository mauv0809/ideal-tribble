package processor

import (
	"time"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/pubsub"
)

// New creates a new Processor.
func New(store Store, notifier Notifier, metrics metrics.Metrics, pubsub pubsub.PubSubClient) *Processor {
	return &Processor{
		store:    store,
		pubsub:   pubsub,
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
		startTime := time.Now()
		p.processMatch(match, dryRun)
		duration := time.Since(startTime).Milliseconds()
		p.metrics.ObserveProcessingDuration(float64(duration))
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
			// Ensure all players from the match are in our database.
			var playersToUpsert []club.PlayerInfo
			for _, team := range match.Teams {
				for _, player := range team.Players {
					playersToUpsert = append(playersToUpsert, club.PlayerInfo{
						ID:    player.UserID,
						Name:  player.Name,
						Level: player.Level,
					})
				}
			}
			if len(playersToUpsert) > 0 {
				if err := p.store.UpsertPlayers(playersToUpsert); err != nil {
					log.Error("Failed to upsert players for match", "error", err, "matchID", match.MatchID)
				}
			}

			// If a match is already played, we never want to send a booking notification.
			switch match.GameStatus {
			case playtomic.GameStatusPlayed:
				// If results are also confirmed, we can jump straight to processing the result.
				switch match.ResultsStatus {
				case playtomic.ResultsStatusConfirmed:
					log.Info("Match is new but already played with confirmed results. Skipping booking notification and advancing to result available.", "matchID", match.MatchID)
					p.updateStatus(match, playtomic.StatusResultAvailable, dryRun)
				case playtomic.ResultsStatusExpired:
					log.Info("Match is new and already played, but results are expired. Setting match to completed.", "matchID", match.MatchID)
					p.updateStatus(match, playtomic.StatusCompleted, dryRun)
				default:
					// If played but results are not ready, just mark booking as "notified" to prevent future booking notifications.
					log.Info("Match is new and already played, but results are not confirmed. Skipping booking notification.", "matchID", match.MatchID)
					p.updateStatus(match, playtomic.StatusBookingNotified, dryRun)
				}
			case playtomic.GameStatusCanceled:
				log.Info("Match is canceled. Setting match to completed.", "matchID", match.MatchID)
				p.updateStatus(match, playtomic.StatusCompleted, dryRun)
			default:
				// This is a normal, upcoming match. Send the booking notification.
				log.Info("Match is new. Sending booking notification.", "matchID", match.MatchID)
				if !dryRun {
					p.pubsub.SendMessage("ball-boy", match)
				}
				p.notifier.SendBookingNotification(match, dryRun)
				p.updateStatus(match, playtomic.StatusBookingNotified, dryRun)
			}

		case playtomic.StatusBookingNotified:
			if match.GameStatus == playtomic.GameStatusPlayed && match.ResultsStatus == playtomic.ResultsStatusConfirmed {
				log.Info("Match has been played. Marking as result available.", "matchID", match.MatchID)
				p.updateStatus(match, playtomic.StatusResultAvailable, dryRun)
			}

		case playtomic.StatusResultAvailable:
			log.Info("Match result is available. Sending result notification.", "matchID", match.MatchID)
			timeEnded := time.Unix(match.End, 0)
			timeSinceEnd := time.Since(timeEnded)
			//If game is ended more than 1 day ago we should not send results and just set update stats. This way we can fetch historic data without sending notifications.
			if timeSinceEnd < 24*time.Hour {
				p.notifier.SendResultNotification(match, dryRun)
			}
			p.updateStatus(match, playtomic.StatusResultNotified, dryRun)

		case playtomic.StatusResultNotified:
			log.Info("Match result has been notified. Updating player stats.", "matchID", match.MatchID)
			if !dryRun {
				p.pubsub.SendMessage("update-player-stats", match)

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
func (p *Processor) UpdatePlayerStats(match *playtomic.PadelMatch) {
	log.Debug("Updating player stats", "matchID", match.MatchID)
	p.store.UpdatePlayerStats(match)
}
func (p *Processor) AssignBallBringer(match *playtomic.PadelMatch, dryRun bool) {
	if match.BallBringerID != "" {
		log.Debug("Ball bringer already assigned", "matchID", match.MatchID, "player", match.BallBringerName)
		return
	}

	var playerIDs []string
	for _, team := range match.Teams {
		for _, player := range team.Players {
			playerIDs = append(playerIDs, player.UserID)
		}
	}

	if len(playerIDs) == 0 {
		log.Warn("No players found in match to assign a ball bringer", "matchID", match.MatchID)
		return
	}

	players, err := p.store.GetPlayers(playerIDs)
	if err != nil {
		log.Error("Failed to get players for ball bringer assignment", "error", err, "matchID", match.MatchID)
		return
	}

	if len(players) == 0 {
		log.Warn("Could not find any of the match players in the database", "matchID", match.MatchID, "playerIDs", playerIDs)
		return
	}

	// Find player with the minimum ball_bringer_count
	var ballBringer club.PlayerInfo
	minCount := -1
	for _, p := range players {
		if minCount == -1 || p.BallBringerCount < minCount {
			minCount = p.BallBringerCount
			ballBringer = p
		}
	}

	log.Info("Assigning ball bringer", "matchID", match.MatchID, "player", ballBringer.Name, "count", ballBringer.BallBringerCount)
	if !dryRun {
		err := p.store.SetBallBringer(match.MatchID, ballBringer.ID, ballBringer.Name)
		if err != nil {
			log.Error("Failed to set ball bringer in store", "error", err, "matchID", match.MatchID)
			return
		}
	} else {
		log.Info("[Dry Run] Would have assigned ball bringer", "player", ballBringer.Name)
	}

	// Update the in-memory match object so the notifier has the correct data
	match.BallBringerID = ballBringer.ID
	match.BallBringerName = ballBringer.Name
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
