package service

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const uvVersion = "0.13.5"

type uvAsset struct{ name, sha256 string }

var uvAssets = map[string]uvAsset{
	"windows/amd64": {"uv-x86_64-pc-windows-msvc.zip", "a38b8d1902b7cfbc76b423ba1bb456f623f42064cdc72628b6075b89afd1019a"},
	"windows/arm64": {"uv-aarch64-pc-windows-msvc.zip", "09f47edf90726084a70f9426f6e3ca4f46e9bf7cbf23b1716ec64d60b055fae3"},
	"darwin/arm64":  {"uv-aarch64-apple-darwin.tar.gz", "743f85035a1996ecb350659e01cd9faea9709829be09b1b4b03c73eaf3c9cd73"},
	"darwin/amd64":  {"uv-x86_64-apple-darwin.tar.gz", "b0f52b0e3945b4107d89024ea38e625683011fbfc05a0d98f9fa2d6dcee28c66"},
	"linux/amd64":   {"uv-x86_64-unknown-linux-gnu.tar.gz", "f68d291c75783122dad58ab77def0e13f3233f358731db5d6d7e8715603b928f"},
	"linux/arm64":   {"uv-aarch64-unknown-linux-gnu.tar.gz", "4bb69cf95ed13f23713bb7bc09f241896ea0c75b7c557e615a20604b82f9c403"},
}

// The checksum is pinned in the app, independently of the downloaded archive.
func downloadVerified(ctx context.Context, client *http.Client, url, digest, destination string, progress func(int64, int64)) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download HTTP %d", response.StatusCode)
	}
	file, err := os.CreateTemp(filepath.Dir(destination), ".download-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	hash := sha256.New()
	var completed int64
	last := time.Time{}
	buffer := make([]byte, 128*1024)
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			if _, err = file.Write(buffer[:n]); err != nil {
				file.Close()
				return err
			}
			hash.Write(buffer[:n])
			completed += int64(n)
			if completed > 128*1024*1024 {
				file.Close()
				return errors.New("bootstrap archive exceeds size limit")
			}
			if time.Since(last) >= 200*time.Millisecond {
				progress(completed, response.ContentLength)
				last = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			file.Close()
			return readErr
		}
	}
	if err = file.Close(); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != digest {
		return errors.New("download checksum mismatch; file was not installed")
	}
	if err = os.Rename(file.Name(), destination); err != nil {
		return err
	}
	progress(completed, completed)
	return nil
}

// Only the expected executable is extracted to a fixed path; archive paths are never used as targets.
func extractUV(archive, destination, executable string) error {
	file, err := os.CreateTemp(filepath.Dir(destination), ".uv-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	var found bool
	copyBinary := func(reader io.Reader) error {
		n, err := io.Copy(file, io.LimitReader(reader, 96*1024*1024+1))
		if err != nil {
			return err
		}
		if n == 0 || n > 96*1024*1024 {
			return errors.New("invalid bootstrap executable size")
		}
		found = true
		return nil
	}
	if strings.HasSuffix(archive, ".zip") {
		reader, err := zip.OpenReader(archive)
		if err != nil {
			return err
		}
		defer reader.Close()
		for _, entry := range reader.File {
			if path.Base(entry.Name) != executable {
				continue
			}
			if !entry.Mode().IsRegular() || strings.Contains(entry.Name, "..") {
				return errors.New("invalid bootstrap archive entry")
			}
			stream, err := entry.Open()
			if err != nil {
				return err
			}
			err = copyBinary(stream)
			stream.Close()
			if err != nil {
				return err
			}
			break
		}
	} else {
		stream, err := os.Open(archive)
		if err != nil {
			return err
		}
		defer stream.Close()
		gzipReader, err := gzip.NewReader(stream)
		if err != nil {
			return err
		}
		defer gzipReader.Close()
		reader := tar.NewReader(gzipReader)
		for {
			entry, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if path.Base(entry.Name) != executable {
				continue
			}
			if entry.Typeflag != tar.TypeReg || strings.Contains(entry.Name, "..") {
				return errors.New("invalid bootstrap archive entry")
			}
			if err = copyBinary(reader); err != nil {
				return err
			}
			break
		}
	}
	if !found {
		return errors.New("bootstrap executable missing from archive")
	}
	if err = file.Chmod(0o700); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), destination)
}

func prepareUV(ctx context.Context, cache string, update func(AISetupStatus)) (string, error) {
	asset, ok := uvAssets[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return "", errors.New("automatic Python setup is not supported on this platform")
	}
	folder := filepath.Join(cache, "bootstrap", "uv-"+uvVersion)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return "", err
	}
	executable := "uv"
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	uv := filepath.Join(folder, executable)
	if info, err := os.Stat(uv); err == nil && info.Size() > 0 {
		return uv, nil
	}
	archive := filepath.Join(folder, asset.name)
	update(AISetupStatus{Phase: "bootstrap", Detail: "ดาวน์โหลดตัวเตรียม Python"})
	err := downloadVerified(ctx, &http.Client{Timeout: 10 * time.Minute},
		"https://github.com/astral-sh/uv/releases/download/"+uvVersion+"/"+asset.name,
		asset.sha256, archive, func(done, total int64) {
			update(AISetupStatus{Phase: "bootstrap", Detail: "ดาวน์โหลดตัวเตรียม Python", Completed: done, Total: total, Unit: "bytes"})
		})
	if err != nil {
		return "", err
	}
	defer os.Remove(archive)
	if err = extractUV(archive, uv, executable); err != nil {
		return "", err
	}
	return uv, nil
}
