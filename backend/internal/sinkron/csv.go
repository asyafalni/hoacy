package sinkron

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CSV reads tabs from <Dir>/<tab>.csv — for tests and for local development
// without Google credentials (export a tab with File → Download → CSV).
type CSV struct{ Dir string }

func (c CSV) Baca(_ context.Context, tab string) ([][]string, error) {
	f, err := os.Open(filepath.Join(c.Dir, filepath.Base(tab)+".csv"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	return r.ReadAll()
}

func (c CSV) DaftarTab(context.Context) ([]string, error) {
	m, err := filepath.Glob(filepath.Join(c.Dir, "*.csv"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range m {
		out = append(out, strings.TrimSuffix(filepath.Base(p), ".csv"))
	}
	sort.Strings(out)
	return out, nil
}
