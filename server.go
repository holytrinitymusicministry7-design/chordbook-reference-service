
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	ug "github.com/Pilfer/ultimate-guitar-scraper/pkg/ultimateguitar"
)

type Candidate struct {
	ID     int64   `json:"id"`
	Song   string  `json:"song"`
	Artist string  `json:"artist"`
	Type   string  `json:"type"`
	Rating float64 `json:"rating"`
	Votes  int64   `json:"votes"`
	Key    string  `json:"key"`
	URL    string  `json:"url"`
}

type SearchResponse struct {
	Source     string      `json:"source"`
	BestMatch  *Candidate  `json:"best_match,omitempty"`
	Chart      string      `json:"chart,omitempty"`
	Candidates []Candidate `json:"candidates"`
}

func main() {
	secret := os.Getenv("SERVICE_TOKEN")
	if secret == "" {
		log.Fatal("SERVICE_TOKEN environment variable is required")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "Use GET",
			})
			return
		}

		if r.Header.Get("X-Service-Token") != secret {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "Unauthorized",
			})
			return
		}

		title := strings.TrimSpace(r.URL.Query().Get("q"))
		if title == "" || len(title) > 180 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "Provide a song title using q",
			})
			return
		}

		scraper := ug.New()
		result, err := scraper.Search(ug.SearchParams{
			Title: title,
			Page:  1,
			Type:  []ug.TabType{ug.TabTypeChords},
		})
		if err != nil {
			log.Printf("Ultimate Guitar search error: %v", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error": "Reference search failed",
			})
			return
		}

		candidates := make([]Candidate, 0, len(result.Tabs))
		for _, tab := range result.Tabs {
			candidates = append(candidates, Candidate{
				ID:     tab.ID,
				Song:   tab.SongName,
				Artist: string(tab.ArtistName),
				Type:   string(tab.Type),
				Rating: tab.Rating,
				Votes:  tab.Votes,
				Key:    string(tab.TonalityName),
			})
		}

		// Rank by rating first, then by number of votes.
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].Rating != candidates[j].Rating {
				return candidates[i].Rating > candidates[j].Rating
			}
			return candidates[i].Votes > candidates[j].Votes
		})

		response := SearchResponse{
			Source:     "Ultimate Guitar",
			Candidates: candidates,
		}

		if len(candidates) > 0 {
			best := candidates[0]
			tab, err := scraper.GetTabByID(best.ID)
			if err != nil {
				log.Printf("Ultimate Guitar chart fetch error: %v", err)
				writeJSON(w, http.StatusBadGateway, map[string]string{
					"error": "Found a chart, but could not fetch its content",
				})
				return
			}

			best.URL = tab.URLWeb
			response.BestMatch = &best
			response.Chart = tab.Content
		}

		writeJSON(w, http.StatusOK, response)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Reference service listening on port %s", port)
	log.Fatal(server.ListenAndServe())
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
