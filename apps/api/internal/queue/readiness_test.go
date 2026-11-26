package queue

import "testing"

func TestParseReadinessStrictBoolean(t *testing.T) {
	for _, body := range []string{`{}`, `null`, `[]`, `{"isReady":null}`, `{"isReady":0}`, `{"isReady":"true"}`, `{"IsReady":true}`, `{"isReady":true,"isReady":false}`, `{"isReady":true,"x":1}`, `{"isReady":true} {}`} {
		if _, err := ParseReadinessRequest([]byte(body)); err == nil {
			t.Errorf("accepted %s", body)
		}
	}
	for _, tc := range []struct {
		body string
		want bool
	}{{`{"isReady":true}`, true}, {` { "isReady" : false } `, false}} {
		if got, err := ParseReadinessRequest([]byte(tc.body)); err != nil || got != tc.want {
			t.Errorf("%s: %v %v", tc.body, got, err)
		}
	}
}
