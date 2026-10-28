package google

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

func UploadToDrive(ctx context.Context, path string) (string, error) {
	ts, err := TokenSource(ctx)
	if err != nil {
		return "", err
	}
	srv, err := drive.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	created, err := srv.Files.Create(&drive.File{Name: filepath.Base(path)}).Media(f).Fields("id, webViewLink").Context(ctx).Do()
	if err != nil {
		if strings.Contains(err.Error(), "insufficient") || strings.Contains(err.Error(), "403") {
			return "", &scopeError{err}
		}
		return "", err
	}
	return created.WebViewLink, nil
}

type scopeError struct{ err error }

func (e *scopeError) Error() string {
	return e.err.Error() + " (run kairo google login again to allow Google Drive uploads)"
}
