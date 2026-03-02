package santricity

import (
	"context"
	"math/rand"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func RandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func LogHTTPRequest(r *http.Request, body []byte) {
	entry := Logc(r.Context()).WithFields(log.Fields{
		"method": r.Method,
		"url":    r.URL.String(),
	})

	if len(body) > 0 {
		// Try to log as string if possible
		entry = entry.WithField("body", string(body))
	}

	entry.Debug("api_request")
}

func LogHTTPResponse(ctx context.Context, r *http.Response, body []byte) {
	entry := Logc(ctx).WithFields(log.Fields{
		"status": r.Status,
		"code":   r.StatusCode,
	})

	if len(body) > 0 {
		// Try to truncate very long bodies
		if len(body) > 4096 {
			entry = entry.WithField("body", string(body[:4096])+"...(truncated)")
		} else {
			entry = entry.WithField("body", string(body))
		}
	}

	entry.Debug("api_response")
}
