package santricity

import (
	"context"
	"math/rand"
	"net/http"
	"time"
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
	// No-op or log
}

func LogHTTPResponse(ctx context.Context, r *http.Response, body []byte) {
	// No-op or log
}
