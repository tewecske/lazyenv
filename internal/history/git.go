package history

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lazynop/lazyenv/internal/config"
	"github.com/lazynop/lazyenv/internal/model"
	"github.com/lazynop/lazyenv/internal/parser"
)

// gitTimeout bounds the whole git history read.
const gitTimeout = 10 * time.Second

type commit struct {
	hash, short, author string
	time                time.Time
}

// Git returns the values each key had in the last maxCommits commits of
// envPath, one entry per change. Returns nil outside a git repository.
func Git(envPath string, maxCommits int, secrets config.SecretsConfig) ([]Entry, error) {
	abs, err := filepath.Abs(envPath)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", envPath, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	dir, name := filepath.Dir(abs), filepath.Base(abs)
	commits, err := gitLog(ctx, dir, name, maxCommits)
	if err != nil || len(commits) == 0 {
		return nil, nil // not a repo or never committed
	}

	var out []Entry
	var prev map[string]model.EnvVar
	for _, c := range slices.Backward(commits) { // oldest first
		data, err := gitCmd(ctx, dir, "show", c.hash+":./"+name)
		if err != nil {
			data = nil // file deleted in this commit
		}
		cur := lastVars(parser.ParseBytes(name, data, secrets).Vars)
		for key, v := range cur {
			if old, ok := prev[key]; ok && old.Value == v.Value {
				continue
			}
			out = append(out, Entry{
				Key: key, Value: v.Value, Time: c.time, Secret: v.IsSecret,
				Source: c.short, Author: c.author,
			})
		}
		prev = cur
	}
	if ctx.Err() != nil {
		return out, fmt.Errorf("reading git history: %w", ctx.Err())
	}
	return out, nil
}

// gitLog lists the commits touching name, newest first.
func gitLog(ctx context.Context, dir, name string, limit int) ([]commit, error) {
	out, err := gitCmd(ctx, dir, "log", "-n", strconv.Itoa(limit), "--format=%H%x1f%h%x1f%at%x1f%an", "--", name)
	if err != nil {
		return nil, err
	}
	var commits []commit
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		f := strings.Split(line, "\x1f")
		if len(f) != 4 {
			continue
		}
		ts, err := strconv.ParseInt(f[2], 10, 64)
		if err != nil {
			continue
		}
		commits = append(commits, commit{hash: f[0], short: f[1], time: time.Unix(ts, 0), author: f[3]})
	}
	return commits, nil
}

func gitCmd(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	return cmd.Output()
}
