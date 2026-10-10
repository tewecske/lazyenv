package history

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/lazynop/lazyenv/internal/model"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func TestReplacedChangedAndDeleted(t *testing.T) {
	before := []model.EnvVar{{Key: "A", Value: "1"}, {Key: "B", Value: "2"}, {Key: "C", Value: "3"}}
	after := []model.EnvVar{{Key: "A", Value: "1"}, {Key: "B", Value: "20"}, {Key: "D", Value: "4"}}

	got := Replaced(before, after, false, t0)

	assert.Equal(t, []Entry{
		{Key: "B", Value: "2", Time: t0, Source: SourceLocal},
		{Key: "C", Value: "3", Time: t0, Source: SourceLocal},
	}, got)
}

func TestReplacedUsesLastDuplicate(t *testing.T) {
	before := []model.EnvVar{{Key: "A", Value: "old"}, {Key: "A", Value: "1"}}
	after := []model.EnvVar{{Key: "A", Value: "1"}}

	assert.Empty(t, Replaced(before, after, false, t0))
}

func TestReplacedSecrets(t *testing.T) {
	before := []model.EnvVar{{Key: "TOKEN", Value: "s1", IsSecret: true}, {Key: "PLAIN", Value: "p1"}}
	after := []model.EnvVar{{Key: "TOKEN", Value: "s2"}, {Key: "PLAIN", Value: "p2", IsSecret: true}}

	assert.Empty(t, Replaced(before, after, false, t0), "secrets are skipped by default")

	got := Replaced(before, after, true, t0)
	assert.Len(t, got, 2)
	for _, e := range got {
		assert.True(t, e.Secret, e.Key)
	}
}

func TestForKey(t *testing.T) {
	entries := []Entry{
		{Key: "A", Value: "1", Time: t0},
		{Key: "B", Value: "x", Time: t0},
		{Key: "A", Value: "2", Time: t0.Add(time.Hour)},
		{Key: "A", Value: "cur", Time: t0.Add(2 * time.Hour)},
	}

	got := ForKey(entries, "A", "cur")

	assert.Equal(t, []string{"2", "1"}, values(got))
}

func values(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Value
	}
	return out
}
