package models

import (
	"testing"
	"time"
)

func TestBaseUserSetters(t *testing.T) {
	u := &BaseUser{}
	u.SetUserID("abc")
	if got := u.GetUserID(); got != "abc" {
		t.Fatalf("expected user id abc, got %s", got)
	}

	before := time.Now().Add(-time.Second)
	u.LastUpdated = before
	u.SetLastUpdated()
	if !u.GetLastUpdated().After(before) {
		t.Fatalf("expected lastUpdated to update, before=%v after=%v", before, u.GetLastUpdated())
	}
}
