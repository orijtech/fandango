package fandango

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type Client struct {
	sync.RWMutex
	version string
	apiKey  string
}

func NewDefaultClient(apiKeysToTry ...string) (*Client, error) {
	client := new(Client)
	client.SetAPIKey(envOrDefault("FANDANGO_API_KEY", apiKeysToTry...))
	if client.APIKey() == "" {
		return nil, errEmptyAPIKey
	}
	return client, nil
}

func (c *Client) SetAPIKey(key string) {
	c.Lock()
	defer c.Unlock()

	c.apiKey = strings.TrimSpace(key)
}

func (c *Client) SetVersion(v string) {
	c.Lock()
	defer c.Unlock()

	c.version = v
}

func envOrDefault(key string, fallbacks ...string) string {
	retr := strings.TrimSpace(os.Getenv(key))
	if retr != "" {
		return retr
	}

	for _, fallback := range fallbacks {
		if retr = strings.TrimSpace(fallback); retr != "" {
			break
		}
	}

	return retr

}

func (c *Client) APIKey() string {
	return c.apiKey
}

const defaultAPIVersion = "1.0"

func (c *Client) APIVersion() string {
	c.RLock()
	defer c.RUnlock()

	if c.version == "" {
		return defaultAPIVersion
	}

	return c.version
}

type Poster map[Size]string
type Size string

const (
	SzUnknown   Size = "unknown"
	SzThumbnail Size = "thumbnail"
	SzProfile   Size = "profile"
	SzOriginal  Size = "original"
)

type Star struct {
	Name       string   `json:"name"`
	Id         string   `json:"id"`
	Characters []string `json:"characters"`
}

type Cast []*Star

type Rating map[string]interface{}

type LinksMap map[string]string

type Movie struct {
	Title            string            `json:"title"`
	Year             int               `json:"year"`
	MPAARating       string            `json:"mpaa_rating"`
	RuntimeMinutes   float32           `json:"runtime"`
	CriticsConsensus string            `json:"critics_consensus"`
	ReleaseDates     map[string]string `json:"release_dates"`
	Ratings          Rating            `json:"ratings"`
	Synopsis         string            `json:"synopsis"`
	Posters          Poster            `json:"posters"`
	AbridgedCast     Cast              `json:"abridged_cast"`
	Links            LinksMap          `json:"links"`
}

type UpcomingMoviesResultPage struct {
	Total        uint     `json:"total"`
	Movies       []*Movie `json:"movies"`
	Links        LinksMap `json:"links"`
	LinkTemplate string   `json:"link_template"`
}

type UpcomingMovieSearch struct {
	ItemsPerPage int             `json:"page_limit"`
	MaxPage      int             `json:"page"`
	Country      string          `json:"country"`
	Cancel       <-chan struct{} `json:"-"`
}

// http://api.rottentomatoes.com/api/public/v1.0/lists/movies/upcoming.json?apikey=[your_api_key]&page_limit=1
const baseURL = "http://api.rottentomatoes.com/api/public"

func (c *Client) makeUpcomingMoviesURL(q *UpcomingMovieSearch) (string, error) {
	values := url.Values{
		"apikey": []string{c.apiKey},
	}
	if q != nil {
		if q.ItemsPerPage > 0 {
			values.Set("page_limit", fmt.Sprintf("%d", q.ItemsPerPage))
		}
		if q.MaxPage > 0 {
			values.Set("page", fmt.Sprintf("%d", q.MaxPage))
		}
		if q.Country != "" {
			values.Set("country", q.Country)
		}
	}

	fullURL := fmt.Sprintf("%s/v%s/lists/movies/upcoming/json?%s", baseURL, c.APIVersion(), values.Encode())
	return fullURL, nil
}

var errEmptyAPIKey = errors.New("empty api key")

func (c *Client) UpcomingMovies(query *UpcomingMovieSearch) (<-chan *UpcomingMoviesResultPage, error) {
	apiKey := c.APIKey()
	if apiKey == "" {
		return nil, errEmptyAPIKey
	}

	dataURL, err := c.makeUpcomingMoviesURL(query)
	// log.Printf("dataURL: %s err: %v\n", dataURL, err)
	if err != nil {
		return nil, err
	}

	pagesChan := make(chan *UpcomingMoviesResultPage)
	go func() {
		defer close(pagesChan)

		throttle := time.NewTicker(1e9)
		working := true
		for working {
			select {
			case _, _ = <-query.Cancel:
				break
			case <-throttle.C:
				res, err := http.Get(dataURL)
				// log.Printf("res: %#v err: %v\n", res, err)
				if err != nil {
					// TODO: handle this error
					working = false
					break
				}
				page, err := parseUpcomingMoviesResponse(res)
				// log.Printf("page: %#v err: %v\n", page, err)
				if err != nil {
					working = false
					// TODO: handle this error
					break
				}

				pagesChan <- page

				// Set to the next page if we have one.
				dataURL = page.Links.GetNextURL()
				// log.Printf("next::dataURL: %s\n", dataURL)
				if dataURL == "" {
					working = false
					break
				}
			}
		}
	}()

	return pagesChan, nil
}

func (l *LinksMap) GetNextURL() string {
	if l == nil {
		return ""
	}

	return (*l)["next"]
}

func statusOK(code int) bool { return code >= 200 && code <= 299 }

func parseUpcomingMoviesResponse(res *http.Response) (*UpcomingMoviesResultPage, error) {
	defer res.Body.Close()

	if !statusOK(res.StatusCode) {
		return nil, fmt.Errorf("%s", res.Status)
	}
	blob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	umpage := new(UpcomingMoviesResultPage)
	if err := json.Unmarshal(blob, umpage); err != nil {
		return nil, err
	}

	return umpage, nil
}
