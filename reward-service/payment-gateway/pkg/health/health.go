package health

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	goredis "github.com/redis/go-redis/v9"
)

type Report struct {
	Status   string    `json:"status"`
	Time     time.Time `json:"time"`
	Database bool      `json:"database"`
	Redis    bool      `json:"redis"`
}

func Check(ctx context.Context, db *sqlx.DB, redis goredis.UniversalClient) Report {
	r := Report{Status: "ready", Time: time.Now().UTC()}
	if db != nil && db.PingContext(ctx) == nil {
		r.Database = true
	}
	if redis != nil && redis.Ping(ctx).Err() == nil {
		r.Redis = true
	}
	if !r.Database || !r.Redis {
		r.Status = "degraded"
	}
	return r
}
