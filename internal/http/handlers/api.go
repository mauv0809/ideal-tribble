package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/club"
)

func ListMembersHandler(store club.ClubStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		refresh := r.URL.Query().Get("refresh") == "true"
		if refresh {
			log.Warn("User triggered manual player refresh. This is deprecated and will be removed.")
		}

		players, err := store.GetAllPlayers()
		if err != nil {
			http.Error(w, "Failed to get players", http.StatusInternalServerError)
			log.Error("Failed to get players from store", "error", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(players); err != nil {
			log.Error("Failed to write response", "error", err)
		}
	}
}

func ListMatchesHandler(store club.ClubStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matches, err := store.GetAllMatches()
		if err != nil {
			http.Error(w, "Failed to get matches", http.StatusInternalServerError)
			log.Error("Failed to get matches from store", "error", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(matches); err != nil {
			log.Error("Failed to encode matches to JSON", "error", err)
		}
	}
}

func LeaderboardHandler(store club.ClubStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := store.GetPlayerStats()
		if err != nil {
			http.Error(w, "Failed to get player stats", http.StatusInternalServerError)
			log.Error("Failed to get player stats from store", "error", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			log.Error("Failed to encode player stats to JSON", "error", err)
		}
	}
}