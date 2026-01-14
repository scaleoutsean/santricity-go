package santricity

import (
	"context"

	log "github.com/sirupsen/logrus"
)

func Logc(ctx context.Context) *log.Entry {
	return log.WithContext(ctx)
}
