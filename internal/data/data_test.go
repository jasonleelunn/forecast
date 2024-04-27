package data

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetch(t *testing.T) {
	fakeResponseBody := `{"fake json string"}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fakeResponseBody)
	}))
	defer ts.Close()

	testURL := ts.URL
	body := Fetch(testURL)

	if body == nil || !bytes.Equal(body, []byte(fakeResponseBody)) {
		t.Fail()
	}
}
