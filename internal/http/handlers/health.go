package handlers

import (
	"fmt"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/club"
)

func HealthCheckHandler(store club.ClubStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Debug("Received health check request")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK!")
	}
}

func ClearStoreHandler(store club.ClubStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.URL.Query().Get("matchID")
		if matchID != "" {
			log.Info("Received request to clear a specific match", "matchID", matchID)
			store.ClearMatch(matchID)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Cleared match %s from store!", matchID)
			log.Info("Successfully cleared match from store", "matchID", matchID)
		} else {
			log.Info("Received request to clear entire store")
			store.Clear()
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "Store cleared!")
			log.Info("Store cleared successfully")
		}
	}
}