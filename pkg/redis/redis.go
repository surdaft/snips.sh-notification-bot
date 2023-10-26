package redis

import (
	"github.com/redis/go-redis/v9"
)

var DefaultClient *redis.Client

func New(redisURI string) *redis.Client {
	if DefaultClient != nil {
		opt, err := redis.ParseURL(redisURI)
		if err != nil {
			panic(err)
		}

		DefaultClient = redis.NewClient(opt)
	}

	return DefaultClient
}
