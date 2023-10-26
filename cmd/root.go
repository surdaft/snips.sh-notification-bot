/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/redis/go-redis/v9"

	pkgRedis "github.com/surdaft/snips.sh-notification-bot/pkg/redis"
	pkgShoutrrr "github.com/surdaft/snips.sh-notification-bot/pkg/shoutrrr"
	"github.com/surdaft/snips.sh-notification-bot/pkg/snips"
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
			ticker := time.NewTicker(time.Second * 30)

			snipsClient := snips.New(viper.GetString("snips-instance-uri"))

			// this will create the default clients
			pkgRedis.New(viper.GetString("redis-uri"))
			pkgShoutrrr.New(viper.GetStringSlice("shoutrrr-uris"))

			for {
				<-ticker.C

				go handleTick(snipsClient)
			}
		},
	}
)

func handleTick(snipsClient *snips.Client) {
	slog.Info("handling tick")

	// grab the last hours worth of snips
	snips := snipsClient.GetSnipsSince(time.Now().Add(time.Hour * -1))

	// now shout about them and stick it in redis
	for _, s := range snips {
		if s.Name == "" {
			slog.Info("no name, skipping", slog.String("ID", s.ID))
			continue
		}

		_, err := pkgRedis.DefaultClient.Get(context.TODO(), s.ID).Result()
		if err != redis.Nil {
			slog.Info("we have already shouted about this one", slog.String("ID", s.ID))
			continue
		}

		errs := pkgShoutrrr.DefaultClient.SendAsync("New snip! **"+s.Name+":** "+viper.GetString("snips-instance-uri")+"/f/"+s.ID, nil)

		go func(e chan error) {
			err := <-e
			if err != nil {
				slog.Error(err.Error())
			}
		}(errs)

		// we only store for 24 hrs since that is way ahead of our lookback
		// but it would mean redis doesn't get hella full either
		pkgRedis.DefaultClient.Set(context.TODO(), s.ID, time.Now().Unix(), time.Hour*24)
	}
}

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
