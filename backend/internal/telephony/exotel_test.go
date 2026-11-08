package telephony

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestExotelChunksFollowItsRules(t *testing.T) {
	for _, size := range []int{1, 319, 320, 3199, 3200, 3201, 10000} {
		chunks := exotelChunks(make([]byte, size))
		total := 0
		for _, c := range chunks {
			if len(c) < 3200 || len(c) > 100*1024 || len(c)%320 != 0 {
				t.Errorf("size %d: chunk of %d bytes breaks the rules", size, len(c))
			}
			total += len(c)
		}
		if total < size {
			t.Errorf("size %d: lost audio, sent %d", size, total)
		}
	}
	if len(exotelChunks(nil)) != 0 {
		t.Error("no audio should send nothing")
	}
}

func exotelStatus(opts Options, path string) int {
	gin.SetMode(gin.TestMode)
	h := &Handler{opts: opts}
	r := gin.New()
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.GET("/ws/exotel", h.RequireExotel, ok)
	r.GET("/ws/exotel/:token", h.RequireExotel, ok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w.Code
}

func TestExotelNeedsItsToken(t *testing.T) {
	on := Options{ExotelToken: "secret"}
	cases := []struct {
		opts Options
		path string
		want int
	}{
		{on, "/ws/exotel/secret", http.StatusOK},
		{on, "/ws/exotel?token=secret", http.StatusOK},
		{on, "/ws/exotel", http.StatusForbidden},
		{on, "/ws/exotel/wrong", http.StatusForbidden},
		{on, "/ws/exotel?token=wrong", http.StatusForbidden},
		{Options{}, "/ws/exotel/secret", http.StatusNotFound},
		{Options{}, "/ws/exotel", http.StatusNotFound},
		{Options{TestMode: true}, "/ws/exotel", http.StatusOK},
	}
	for _, c := range cases {
		if got := exotelStatus(c.opts, c.path); got != c.want {
			t.Errorf("%s with %+v: got %d, want %d", c.path, c.opts, got, c.want)
		}
	}
}
