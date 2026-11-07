// Package google talks to Drive and Sheets as the RT's own Google account
// (one OAuth refresh token — a service account has no Drive storage quota on a
// personal account). Drive keeps the proof photos; Sheets is only read: the
// manual sync source and the M-Opex / M-SaldoAwal snapshots.
package google

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	goog "golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/asyafalni/hoacy/backend/internal/berkas"
)

// Config comes from env: GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET,
// GOOGLE_REFRESH_TOKEN (backend/cmd/google-login prints it), DRIVE_FOLDER_ID,
// SHEET_ID.
type Config struct {
	ClientID, ClientSecret, RefreshToken string
	FolderID                             string
	SheetID                              string
}

func (c Config) Lengkap() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.RefreshToken != ""
}

// Scopes: drive.file only reaches files this app created (our uploads);
// readonly Drive is needed for the bukti the old Google Form stored, and
// readonly Sheets for the sync. Nothing here can change the Sheet.
var Scopes = []string{
	drive.DriveFileScope,
	drive.DriveReadonlyScope,
	sheets.SpreadsheetsReadonlyScope,
}

func OAuth(c Config) *oauth2.Config {
	return &oauth2.Config{ClientID: c.ClientID, ClientSecret: c.ClientSecret, Endpoint: goog.Endpoint,
		Scopes: Scopes, RedirectURL: "http://127.0.0.1:8085/callback"}
}

type Klien struct {
	cfg    Config
	drive  *drive.Service
	sheets *sheets.Service
}

func New(ctx context.Context, c Config) (*Klien, error) {
	if !c.Lengkap() {
		return nil, errors.New("google: GOOGLE_CLIENT_ID/SECRET/REFRESH_TOKEN belum diisi")
	}
	ts := OAuth(c).TokenSource(ctx, &oauth2.Token{RefreshToken: c.RefreshToken})
	hc := oauth2.NewClient(ctx, ts)
	hc.Timeout = 30 * time.Second
	d, err := drive.NewService(ctx, option.WithHTTPClient(hc))
	if err != nil {
		return nil, err
	}
	s, err := sheets.NewService(ctx, option.WithHTTPClient(hc))
	if err != nil {
		return nil, err
	}
	return &Klien{cfg: c, drive: d, sheets: s}, nil
}

// ── Drive (berkas.Berkas) ──────────────────────────────────────────────────

func (k *Klien) Simpan(ctx context.Context, nama, mime string, data []byte) (string, error) {
	f := &drive.File{Name: nama, MimeType: mime}
	if k.cfg.FolderID != "" {
		f.Parents = []string{k.cfg.FolderID}
	}
	out, err := k.drive.Files.Create(f).Media(bytes.NewReader(data)).Fields("id").Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("drive: simpan: %w", err)
	}
	return "drive:" + out.Id, nil
}

func (k *Klien) Buka(ctx context.Context, ref string) ([]byte, string, error) {
	id, ok := berkas.IDDrive(ref)
	if !ok {
		return nil, "", fmt.Errorf("drive: bukan referensi Drive: %q", ref)
	}
	res, err := k.drive.Files.Get(id).SupportsAllDrives(true).Context(ctx).Download()
	if err != nil {
		return nil, "", fmt.Errorf("drive: buka: %w", err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, berkas.MaksUkuran+1))
	if err != nil {
		return nil, "", err
	}
	t, _ := berkas.Periksa(data)
	return data, t, nil
}

// ── Sheets (read only) ─────────────────────────────────────────────────────

// Baca returns a tab's cells as text, header row included. Numbers come
// unformatted (300000, never "300,000" or "Rp300.000"), dates and times as the
// Sheet displays them; empty trailing cells are dropped by the API.
func (k *Klien) Baca(ctx context.Context, tab string) ([][]string, error) {
	if k.cfg.SheetID == "" {
		return nil, errors.New("sheets: SHEET_ID belum diisi")
	}
	res, err := k.sheets.Spreadsheets.Values.Get(k.cfg.SheetID, "'"+tab+"'").
		ValueRenderOption("UNFORMATTED_VALUE").DateTimeRenderOption("FORMATTED_STRING").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("sheets: baca %s: %w", tab, err)
	}
	out := make([][]string, len(res.Values))
	for i, row := range res.Values {
		out[i] = make([]string, len(row))
		for j, v := range row {
			if f, ok := v.(float64); ok {
				out[i][j] = strconv.FormatFloat(f, 'f', -1, 64) // a phone typed as a number stays whole
			} else {
				out[i][j] = fmt.Sprint(v)
			}
		}
	}
	return out, nil
}

// DaftarTab lists the spreadsheet's tab names.
func (k *Klien) DaftarTab(ctx context.Context) ([]string, error) {
	if k.cfg.SheetID == "" {
		return nil, errors.New("sheets: SHEET_ID belum diisi")
	}
	ss, err := k.sheets.Spreadsheets.Get(k.cfg.SheetID).Fields("sheets.properties.title").Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, s := range ss.Sheets {
		out = append(out, s.Properties.Title)
	}
	return out, nil
}
