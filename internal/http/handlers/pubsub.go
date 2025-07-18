package handlers

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/processor"
	"github.com/mauv0809/ideal-tribble/internal/pubsub"
)

func BallBoyHandler(processor *processor.Processor, pubsubClient pubsub.PubSubClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Error("Failed to read request body", "error", err)
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		log.Debug("Received ball boy message", "body", string(bodyBytes))

		var pubsubMsg struct {
			Subscription string `json:"subscription"`
			Message      struct {
				Data string `json:"data"`
			} `json:"message"`
		}

		if err := json.Unmarshal(bodyBytes, &pubsubMsg); err != nil {
			log.Error("Failed to unmarshal wrapper JSON", "error", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		rawData, err := base64.StdEncoding.DecodeString(pubsubMsg.Message.Data)
		if err != nil {
			log.Error("Failed to decode base64 data", "error", err)
			http.Error(w, "Invalid base64 data", http.StatusBadRequest)
			return
		}
		isDryRun := IsDryRunFromContext(r)
		match := playtomic.PadelMatch{}
		pubsubClient.ProcessMessage(rawData, &match)
		processor.AssignBallBringer(&match, isDryRun)
		w.Write([]byte("OK"))
	}
}

func UpdatePlayerStatsHandler(processor *processor.Processor, pubsubClient pubsub.PubSubClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Error("Failed to read request body", "error", err)
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		log.Debug("Received update player stats message", "body", string(bodyBytes))

		var pubsubMsg struct {
			Subscription string `json:"subscription"`
			Message      struct {
				Data string `json:"data"`
			} `json:"message"`
		}

		if err := json.Unmarshal(bodyBytes, &pubsubMsg); err != nil {
			log.Error("Failed to unmarshal wrapper JSON", "error", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		rawData, err := base64.StdEncoding.DecodeString(pubsubMsg.Message.Data)
		if err != nil {
			log.Error("Failed to decode base64 data", "error", err)
			http.Error(w, "Invalid base64 data", http.StatusBadRequest)
			return
		}
		isDryRun := IsDryRunFromContext(r)
		match := playtomic.PadelMatch{}
		pubsubClient.ProcessMessage(rawData, &match)
		processor.UpdatePlayerStats(&match, isDryRun)
		w.Write([]byte("OK"))
	}
}

func NotifyBookingHandler(processor *processor.Processor, pubsubClient pubsub.PubSubClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Error("Failed to read request body", "error", err)
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		log.Debug("Recieved notify booking message", "body", string(bodyBytes))

		var pubsubMsg struct {
			Subscription string `json:"subscription"`
			Message      struct {
				Data string `json:"data"`
			} `json:"message"`
		}

		if err := json.Unmarshal(bodyBytes, &pubsubMsg); err != nil {
			log.Error("Failed to unmarshal wrapper JSON", "error", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		rawData, err := base64.StdEncoding.DecodeString(pubsubMsg.Message.Data)
		if err != nil {
			log.Error("Failed to decode base64 data", "error", err)
			http.Error(w, "Invalid base64 data", http.StatusBadRequest)
			return
		}
		isDryRun := IsDryRunFromContext(r)
		match := playtomic.PadelMatch{}
		pubsubClient.ProcessMessage(rawData, &match)
		err = processor.NotifyBooking(&match, isDryRun)
		if err != nil {
			log.Error("Failed to notify booking", "error", err)
			http.Error(w, "Failed to notify booking", http.StatusInternalServerError)
			return
		}
		w.Write([]byte("OK"))
	}
}

func NotifyResultHandler(processor *processor.Processor, pubsubClient pubsub.PubSubClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Error("Failed to read request body", "error", err)
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		log.Debug("Recieved notify booking message", "body", string(bodyBytes))

		var pubsubMsg struct {
			Subscription string `json:"subscription"`
			Message      struct {
				Data string `json:"data"`
			} `json:"message"`
		}

		if err := json.Unmarshal(bodyBytes, &pubsubMsg); err != nil {
			log.Error("Failed to unmarshal wrapper JSON", "error", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		rawData, err := base64.StdEncoding.DecodeString(pubsubMsg.Message.Data)
		if err != nil {
			log.Error("Failed to decode base64 data", "error", err)
			http.Error(w, "Invalid base64 data", http.StatusBadRequest)
			return
		}
		isDryRun := IsDryRunFromContext(r)
		match := playtomic.PadelMatch{}
		pubsubClient.ProcessMessage(rawData, &match)
		err = processor.NotifyResult(&match, isDryRun)
		if err != nil {
			log.Error("Failed to notify result", "error", err)
			http.Error(w, "Failed to notify result", http.StatusInternalServerError)
			return
		}
		w.Write([]byte("OK"))
	}
}
