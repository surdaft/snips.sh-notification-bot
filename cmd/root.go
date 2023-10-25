/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/containrrr/shoutrrr"
	shoutrrrRouter "github.com/containrrr/shoutrrr/pkg/router"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// rootCmd represents the base command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:   "snips.sh-notification-bot",
		Short: "Listen to a snips and shout about new snips using shoutrrr",
		Run: func(cmd *cobra.Command, args []string) {
			// cool, we need to just sit, listen and post about any snips posted
			// recently
			slog.Info("starting listener for: " + viper.GetString("snips-instance-uri"))
			ticker := time.NewTicker(time.Second * 5)
			for {
				<-ticker.C

				slog.Info("we got a tick, yaaas")
				go handleTick()
			}
		},
	}

	redisClient  *redis.Client
	senderClient *shoutrrrRouter.ServiceRouter
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().String("snips-instance-uri", "snips.sh", "Snips.sh URL")
	rootCmd.Flags().String("redis-uri", "redis://127.0.0.1:6379/0", "Redis connection string")
	rootCmd.Flags().StringArray("shoutrrr-uris", []string{}, "Shoutrrr paths to notify")

	viper.BindPFlag("snips-instance-uri", rootCmd.Flags().Lookup("snips-instance-uri"))
	viper.BindPFlag("redis-uri", rootCmd.Flags().Lookup("redis-uri"))
	viper.BindPFlag("shoutrrr-uris", rootCmd.Flags().Lookup("shoutrrr-uris"))

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/snips-notifications")
	viper.AddConfigPath("$HOME/.snips-notifications")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Error(err.Error())
		}
	}
}

func getRedisClient() redis.Client {
	if redisClient == nil {
		opt, err := redis.ParseURL(viper.GetString("redis-uri"))
		if err != nil {
			panic(err)
		}

		redisClient = redis.NewClient(opt)
	}

	return *redisClient
}

func getSender() *shoutrrrRouter.ServiceRouter {
	if senderClient == nil {
		slog.Info(strings.Join(viper.GetStringSlice("shoutrrr-uris"), ", "))
		sender, err := shoutrrr.CreateSender(viper.GetStringSlice("shoutrrr-uris")...)
		if err != nil {
			panic(err)
		}

		senderClient = sender
	}

	return senderClient
}

func handleTick() {
	redisClient := getRedisClient()
	sender := getSender()

	slog.Info("handling tick")

	// grab the last hours worth of snips
	snips := getSnipsSince(time.Now().Add(time.Hour * -1))

	// now shout about them and stick it in redis
	for _, s := range snips {
		if s.Name == "" {
			slog.Info("no name, skipping", slog.String("ID", s.ID))
			continue
		}

		_, err := redisClient.Get(context.TODO(), s.ID).Result()
		if err != redis.Nil {
			slog.Info("we have already shouted about this one", slog.String("ID", s.ID))
			continue
		}

		errs := sender.SendAsync("New snip! **"+s.Name+":** "+viper.GetString("snips-instance-uri")+"/f/"+s.ID, nil)

		go func(e chan error) {
			err := <-e
			if err != nil {
				slog.Error(err.Error())
			}
		}(errs)

		// we only store for 24 hrs since that is way ahead of our lookback
		// but it would mean redis doesn't get hella full either
		redisClient.Set(context.TODO(), s.ID, time.Now().Unix(), time.Hour*24)
	}
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

func getSnips(page int) []Snip {
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

func getSnipsSince(since time.Time) []Snip {
	// ok we need to iterate through pages until we either reach the since, or
	// until we run out of snips

	snips := make([]Snip, 0)
	currPage := 0

	// loop through indefinitely
	for {
		// grab this page
		ss := getSnips(currPage)
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
