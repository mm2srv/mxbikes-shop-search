package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	baseURL             = "https://mxbikes-shop.com"
	startURL            = "https://mxbikes-shop.com/downloads/category/mods/tracks/"
	completeJSONFile    = "mxbikes-shop-tracks.json"
	processedTracksFile = "processed_tracks.json"
	maxRetries          = 3
	retryDelay          = 5 * time.Second
)

// TrackMetadata holds the structured data for a track
type TrackMetadata struct {
	TrackName          string `json:"track_name"`
	TrackURL           string `json:"track_url"`
	AuthorName         string `json:"author_name"`
	AuthorURL          string `json:"author_url"`
	Price              string `json:"price"`
	ReleasedDate       string `json:"released_date"`
	LastUpdated        string `json:"last_updated"`
	FileSize           string `json:"file_size"`
	Version            string `json:"version"`
	ServerVersionURL   string `json:"server_version_url,omitempty"`
	InGameName         string `json:"ingame_mod_name,omitempty"`
	Difficulty         string `json:"difficulty,omitempty"`
	CompatibleWithBeta string `json:"compatible_with_beta,omitempty"`
	ScrapedTimestamp   string `json:"scraped_timestamp"`
}

var client = &http.Client{
	Timeout: 30 * time.Second,
}

func fetchPage(url string) (*goquery.Document, error) {
	var err error

	for i := 0; i < maxRetries; i++ {
		log.Printf("Fetching URL: %s (Attempt %d/%d)", url, i+1, maxRetries)
		req, errHttpNewRequest := http.NewRequest("GET", url, nil)
		if errHttpNewRequest != nil {
			err = fmt.Errorf("failed to create request for %s: %w", url, errHttpNewRequest)
			time.Sleep(retryDelay)
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

		resp, errHttpDo := client.Do(req)
		if errHttpDo != nil {
			err = fmt.Errorf("failed to fetch %s: %w", url, errHttpDo)
			time.Sleep(retryDelay)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("failed to fetch %s: status code %d", url, resp.StatusCode)
			time.Sleep(retryDelay)
			continue
		}

		doc, errGoquery := goquery.NewDocumentFromReader(resp.Body)
		if errGoquery != nil {
			err = fmt.Errorf("failed to parse HTML from %s: %w", url, errGoquery)
			time.Sleep(retryDelay)
			continue
		}
		return doc, nil // Success
	}
	return nil, err // All retries failed
}

func extractTextFromReleaseInfo(doc *goquery.Document, labelText string) string {
	var value string
	doc.Find("ul.release-info li.release-info-block").EachWithBreak(func(i int, s *goquery.Selection) bool {
		label := strings.TrimSpace(s.Find(".rel-info-tag").Text())
		if strings.EqualFold(strings.TrimSpace(label), labelText) {
			value = strings.TrimSpace(s.Find(".rel-info-value p").First().Text())
			if labelText == "Server Version" {
				href, exists := s.Find(".rel-info-value p a").Attr("href")
				if exists {
					value = href
				} else {
					value = ""
				}
			}
			return false
		}
		return true
	})
	return value
}

func extractTextFromDisplayTable(doc *goquery.Document, rowID string) string {
	return strings.TrimSpace(doc.Find(fmt.Sprintf("tr#%s td.fes-display-field-values", rowID)).First().Text())
}

func parseTrackDetailPage(trackURL string) (*TrackMetadata, error) {
	doc, err := fetchPage(trackURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching detail page %s: %w", trackURL, err)
	}

	meta := &TrackMetadata{TrackURL: trackURL}

	meta.TrackName = strings.TrimSpace(doc.Find("h1.single-post-title").First().Text())
	meta.AuthorName = strings.TrimSpace(doc.Find(".single--post--content a[href*='/creator/']").First().Text())
	meta.AuthorURL, _ = doc.Find(".single--post--content a[href*='/creator/']").First().Attr("href")
	if !strings.HasPrefix(meta.AuthorURL, "http") && meta.AuthorURL != "" {
		meta.AuthorURL = baseURL + meta.AuthorURL
	}

	priceText := strings.TrimSpace(doc.Find(".product-purchase-box .edd_price").First().Text())
	if priceText == "" {
		priceText = strings.TrimSpace(doc.Find(".release-info-block .edd_price").First().Text())
	}
	if priceText == "" {
		priceText = strings.TrimSpace(doc.Find(".product_widget_inside p:contains('Free')").First().Text())
		if !strings.EqualFold(priceText, "Free") {
			priceText = ""
		}
	}
	meta.Price = priceText

	meta.ReleasedDate = extractTextFromReleaseInfo(doc, "Released")
	meta.LastUpdated = extractTextFromReleaseInfo(doc, "Last Updated")
	meta.FileSize = extractTextFromReleaseInfo(doc, "File Size")
	meta.Version = extractTextFromReleaseInfo(doc, "Version")
	meta.ServerVersionURL = extractTextFromReleaseInfo(doc, "Server Version")

	meta.InGameName = extractTextFromDisplayTable(doc, "ingame_mod_name")
	meta.Difficulty = extractTextFromDisplayTable(doc, "mod_difficulty")
	meta.CompatibleWithBeta = extractTextFromDisplayTable(doc, "compatible_with")

	meta.ScrapedTimestamp = time.Now().Format(time.RFC3339)

	if meta.TrackName == "" {
		log.Printf("Warning: Could not parse track name for URL %s", trackURL)
	}

	return meta, nil
}

func loadProcessedTracks() (map[string]bool, error) {
	processed := make(map[string]bool)
	filePath := processedTracksFile
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return processed, nil
	}

	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading processed tracks file %s: %w", filePath, err)
	}
	if len(bytes) == 0 {
		return processed, nil
	}

	var urls []string
	if err := json.Unmarshal(bytes, &urls); err != nil {
		log.Printf("Warning: Unmarshal for processed_tracks.json failed. Starting fresh. Error: %v", err)
		return make(map[string]bool), nil
	}

	for _, u := range urls {
		processed[u] = true
	}
	return processed, nil
}

func saveProcessedTracks(processed map[string]bool) error {
	var urls []string
	for u := range processed {
		urls = append(urls, u)
	}

	sort.Strings(urls)

	bytes, err := json.MarshalIndent(urls, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling processed tracks: %w", err)
	}
	return ioutil.WriteFile(processedTracksFile, bytes, 0644)
}

func loadCompleteTracks() (map[string]TrackMetadata, error) {
	allTracks := make(map[string]TrackMetadata)
	filePath := completeJSONFile

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return allTracks, nil
	}

	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading complete JSON file %s: %w", filePath, err)
	}
	if len(bytes) == 0 {
		return allTracks, nil
	}

	var trackList []TrackMetadata
	if err := json.Unmarshal(bytes, &trackList); err != nil {
		return nil, fmt.Errorf("error unmarshalling complete JSON file %s: %w", filePath, err)
	}
	for _, track := range trackList {
		allTracks[track.TrackURL] = track
	}
	return allTracks, nil
}

// ByReleaseAndScrape implements sort.Interface for []TrackMetadata based on
// ReleasedDate (descending) and then ScrapedTimestamp (descending).
type ByReleaseAndScrape []TrackMetadata

func (a ByReleaseAndScrape) Len() int      { return len(a) }
func (a ByReleaseAndScrape) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByReleaseAndScrape) Less(i, j int) bool {
	// Parse ReleasedDate
	layoutReleased := "January 2, 2006"
	t1Released, err1 := time.Parse(layoutReleased, a[i].ReleasedDate)
	t2Released, err2 := time.Parse(layoutReleased, a[j].ReleasedDate)

	// Handle parsing errors (e.g., by treating unparseable dates as "older")
	// A more robust solution might involve error handling or logging
	if err1 != nil && err2 == nil {
		return false // a[i] is "older" or invalid
	}
	if err1 == nil && err2 != nil {
		return true // a[j] is "older" or invalid
	}
	if err1 != nil && err2 != nil {
		// If both are invalid, fall back to ScrapedTimestamp or treat as equal for ReleasedDate
		// For simplicity here, we proceed to ScrapedTimestamp comparison
	}

	// Compare ReleasedDate (descending)
	if err1 == nil && err2 == nil && !t1Released.Equal(t2Released) {
		return t1Released.After(t2Released) // Sort by latest release first
	}

	// If ReleasedDate is the same (or both unparseable), sort by ScrapedTimestamp (descending)
	// Assuming RFC3339 format like "2006-01-02T15:04:05Z07:00"
	t1Scraped, err1Scraped := time.Parse(time.RFC3339, a[i].ScrapedTimestamp)
	t2Scraped, err2Scraped := time.Parse(time.RFC3339, a[j].ScrapedTimestamp)

	if err1Scraped != nil && err2Scraped == nil {
		return false
	}
	if err1Scraped == nil && err2Scraped != nil {
		return true
	}
	if err1Scraped != nil && err2Scraped != nil {
		return false // Treat as equal if both timestamps are invalid
	}

	return t1Scraped.After(t2Scraped) // Sort by latest scrape first
}

func saveCompleteTracks(allTracksMap map[string]TrackMetadata) error {
	var trackList []TrackMetadata
	for _, track := range allTracksMap {
		trackList = append(trackList, track)
	}

	sort.Sort(ByReleaseAndScrape(trackList))

	bytes, err := json.MarshalIndent(trackList, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling complete JSON: %w", err)
	}
	return ioutil.WriteFile(completeJSONFile, bytes, 0644)
}

func main() {
	processedTrackURLs, err := loadProcessedTracks()
	if err != nil {
		log.Printf("Warning: Could not load processed tracks file: %v. Proceeding with empty set.", err)
		processedTrackURLs = make(map[string]bool)
	}

	allTracksMap, err := loadCompleteTracks()
	if err != nil {
		log.Fatalf("Failed to load complete tracks data from %s: %v", completeJSONFile, err)
	}

	currentPageURL := startURL
	stopPagination := false
	newTracksFoundThisRun := 0

	resultsChan := make(chan *TrackMetadata, 50)

	// This collector goroutine will run until resultsChan is closed
	var collectorWg sync.WaitGroup
	collectorWg.Add(1)
	go func() {
		defer collectorWg.Done()
		for trackMeta := range resultsChan {
			if trackMeta == nil {
				continue
			}
			allTracksMap[trackMeta.TrackURL] = *trackMeta // Add or update
			processedTrackURLs[trackMeta.TrackURL] = true
		}
	}()

	for currentPageURL != "" && !stopPagination {
		doc, errFetch := fetchPage(currentPageURL)
		if errFetch != nil {
			log.Printf("Error fetching list page %s: %v. Attempting to find next page.", currentPageURL, errFetch)
			// Try to find next page link even if current page fetch fails
			if doc != nil { // doc might be nil if fetchPage returned error early
				nextPageLink := doc.Find("a.next.page-numbers").First()
				if nextPageLink.Length() > 0 {
					currentPageURL, _ = nextPageLink.Attr("href")
					if !strings.HasPrefix(currentPageURL, "http") && currentPageURL != "" {
						currentPageURL = baseURL + currentPageURL
					}
				} else {
					currentPageURL = ""
				}
			} else {
				currentPageURL = "" // Cannot proceed if doc is nil
			}
			if currentPageURL == "" {
				log.Println("Could not determine next page after error. Stopping.")
				break
			}
			continue
		}

		log.Printf("Processing page: %s", currentPageURL)

		foundTracksOnPage := false
		doc.Find("div.product-grid").EachWithBreak(func(i int, s *goquery.Selection) bool {
			foundTracksOnPage = true
			trackAnchor := s.Find("h4.product-title a").First()
			trackName := strings.TrimSpace(trackAnchor.Text())
			partialTrackURL, exists := trackAnchor.Attr("href")

			if !exists || trackName == "" || partialTrackURL == "" {
				log.Println("Skipping item on list page: missing name or URL.")
				return true // continue to next item
			}

			fullTrackURL := partialTrackURL
			if !strings.HasPrefix(fullTrackURL, "http") {
				fullTrackURL = baseURL + fullTrackURL
			}

			if _, alreadyProcessed := processedTrackURLs[fullTrackURL]; alreadyProcessed {
				log.Printf("Track '%s' (%s) is in processed_tracks.json. Assuming handled.", trackName, fullTrackURL)

				if i == 0 { // If the very first track on the page is already processed
					log.Println("First track on page already processed. Stopping pagination.")
					stopPagination = true
					return false // Break from EachWithBreak
				}
				return true
			}

			// If we are here, this track was NOT in the initial processedTrackURLs
			newTracksFoundThisRun++

			log.Printf("Fetching details for new track: %s (%s)", trackName, fullTrackURL)
			trackMeta, errDetail := parseTrackDetailPage(fullTrackURL)
			if errDetail != nil {
				log.Printf("Error parsing detail page for %s (%s): %v", trackName, fullTrackURL, errDetail)
				return true
			}
			if trackMeta.TrackName == "" {
				log.Printf("Skipping track with no name from URL %s after detail parse", fullTrackURL)
				return true
			}
			resultsChan <- trackMeta // Send successfully parsed metadata

			time.Sleep(1 * time.Second)

			return true // Continue to next track on the page
		})

		if !foundTracksOnPage {
			log.Println("No track items found on page:", currentPageURL, ". Stopping pagination.")
			stopPagination = true
		}

		if stopPagination {
			log.Println("StopPagination flag is true. Breaking from page loop.")
			break
		}

		// Find next page URL
		nextPageLink := doc.Find("a.next.page-numbers").First()
		if nextPageLink.Length() > 0 {
			nextHref, _ := nextPageLink.Attr("href")
			if nextHref == "" {
				currentPageURL = ""
				log.Println("No next page href found on link.")
			} else {
				if !strings.HasPrefix(nextHref, "http") {
					currentPageURL = baseURL + nextHref
				} else {
					currentPageURL = nextHref
				}
			}
		} else {
			log.Println("No 'next page' link found on page.")
			currentPageURL = ""
		}

		if currentPageURL != "" {
			log.Printf("Next page: %s", currentPageURL)
			time.Sleep(1 * time.Second)
		}
	}

	log.Println("Waiting for all track detail scraping goroutines to complete...")
	close(resultsChan) // Close the channel to signal the collector goroutine
	collectorWg.Wait() // Wait for the collector goroutine to finish processing all items

	log.Printf("Found and processed %d new tracks in this run.", newTracksFoundThisRun)

	if err := saveCompleteTracks(allTracksMap); err != nil {
		log.Fatalf("Error saving complete tracks JSON to %s: %v", completeJSONFile, err)
	}
	log.Printf("Successfully saved complete track list to %s", completeJSONFile)

	if err := saveProcessedTracks(processedTrackURLs); err != nil {
		log.Fatalf("Error saving processed tracks list to %s: %v", processedTracksFile, err)
	}
	log.Printf("Successfully saved processed track URLs to %s", processedTracksFile)

	log.Println("Scraping finished.")
}
