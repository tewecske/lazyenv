// Package history records and reads past values of env variables, from a
// local store written on save and from git commits.
package history

import (
	"slices"
	"strings"
	"time"

	"github.com/lazynop/lazyenv/internal/model"
)

// SourceLocal marks entries recorded by lazyenv on save.
const SourceLocal = "local"

// Entry is one past value of a variable.
type Entry struct {
	Key    string    `json:"key"`
	Value  string    `json:"value"`
	Time   time.Time `json:"time"`             // replaced at (local) or committed at (git)
	Secret bool      `json:"secret,omitempty"` // value was detected as a secret
	Source string    `json:"-"`                // SourceLocal or a short commit hash
	Author string    `json:"-"`                // commit author (git only)
}

// Replaced returns the old values of keys changed or deleted between before
// and after. Secrets are skipped unless recordSecrets is set.
func Replaced(before, after []model.EnvVar, recordSecrets bool, now time.Time) []Entry {
	newValues := lastVars(after)
	var out []Entry
	for key, old := range lastVars(before) {
		if v, ok := newValues[key]; ok && v.Value == old.Value {
			continue
		}
		secret := old.IsSecret || newValues[key].IsSecret
		if secret && !recordSecrets {
			continue
		}
		out = append(out, Entry{Key: key, Value: old.Value, Time: now, Secret: secret, Source: SourceLocal})
	}
	slices.SortFunc(out, func(a, b Entry) int { return strings.Compare(a.Key, b.Key) })
	return out
}

// ForKey returns the entries for key, newest first, skipping the current
// value since restoring it would be a no-op.
func ForKey(entries []Entry, key, current string) []Entry {
	var out []Entry
	for _, e := range entries {
		if e.Key == key && e.Value != current {
			out = append(out, e)
		}
	}
	slices.SortStableFunc(out, func(a, b Entry) int { return b.Time.Compare(a.Time) })
	return out
}

// lastVars maps each key to its last occurrence (shell semantics).
func lastVars(vars []model.EnvVar) map[string]model.EnvVar {
	m := make(map[string]model.EnvVar, len(vars))
	for _, v := range vars {
		m[v.Key] = v
	}
	return m
}
