package snips

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/sagikazarmark/slog-shim"
	"github.com/spf13/viper"
)

type Client struct {
	InstanceURI string
}

type Snip struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Size        int    `json:"size"`
	Private     bool   `json:"private"`
	Type        string `json:"type"`
	UserID      string `json:"user_id"`
}

func New(instanceURI string) *Client {
	return &Client{
		InstanceURI: instanceURI,
	}
}

func (c Client) getPage(page int) []Snip {
	snips := make([]Snip, 0)

	client := http.DefaultClient
	snipsApiUri, err := url.Parse(viper.GetString("snips-instance-uri"))
	if err != nil {
		slog.Error(err.Error())
		return snips
	}

	pageStr := strconv.Itoa(page)

	query := snipsApiUri.Query()
	query.Add("page", pageStr)

	snipsApiUri = snipsApiUri.JoinPath("/api/v1/feed")
	snipsApiUri.RawQuery = query.Encode()

	slog.Info("requesting", slog.String("uri", snipsApiUri.String()))

	req, err := http.NewRequest("GET", snipsApiUri.String(), nil)
	if err != nil {
		slog.Error(err.Error())
		return snips
	}

	resp, err := client.Do(req)
	if err != nil {
		slog.Error(err.Error())
		return snips
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Error("non 200 response", slog.Int("status-code", resp.StatusCode))
		return snips
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error(err.Error())
		return snips
	}

	json.Unmarshal(respBody, &snips)
	return snips
}

func (c Client) GetSnipsSince(since time.Time) []Snip {
	// ok we need to iterate through pages until we either reach the since, or
	// until we run out of snips

	snips := make([]Snip, 0)
	currPage := 0

	// loop through indefinitely
	for {
		// grab this page
		ss := c.getPage(currPage)
		shouldBreak := false

		if len(ss) == 0 {
			break
		}

		for _, s := range ss {
			c := time.Now()
			err := c.UnmarshalText([]byte(s.CreatedAt))
			if err != nil {
				slog.Error(err.Error())
				continue
			}

			// if we reached the time limit then force a break out completely
			if c.Before(since) {
				shouldBreak = true
				break
			}

			// add this snip, its within the timeframe
			snips = append(snips, s)
		}

		// break out them all
		if shouldBreak {
			break
		}
	}

	return snips
}
