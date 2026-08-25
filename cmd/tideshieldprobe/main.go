package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type probeResult struct {
	Path        string `json:"path"`
	Status      int    `json:"status"`
	ContentType string `json:"content_type"`
	Bytes       int64  `json:"bytes"`
	Marker      bool   `json:"marker"`
}

type report struct {
	Address string        `json:"address"`
	Results []probeResult `json:"results"`
	Status  string        `json:"status"`
}

func main() {
	goExecutable := flag.String("go", `C:\Program Files\Go\bin\go.exe`, "Go executable")
	address := flag.String("addr", "127.0.0.1:21226", "TideShield address")
	dataDir := flag.String("data", filepath.Join(os.TempDir(), "tideshield-probe"), "probe data directory")
	webDir := flag.String("web", "./web", "web asset directory")
	flag.Parse()
	if err := run(*goExecutable, *address, *dataDir, *webDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(goExecutable, address, dataDir, webDir string) error {
	if _, err := net.ResolveTCPAddr("tcp", address); err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, goExecutable, "run", "./cmd/tideshield", "-addr", address, "-data", dataDir, "-web", webDir)
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return err
	}
	defer func() {
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		_ = command.Wait()
	}()
	base := "http://" + address
	client := &http.Client{Timeout: 4 * time.Second}
	if err := waitReady(ctx, client, base+"/healthz"); err != nil {
		return err
	}
	paths := []string{
		"/healthz",
		"/operations.html",
		"/equipment.html",
		"/interlocks.html",
		"/incidents.html",
		"/api/operations",
		"/api/equipment",
		"/api/interlocks",
		"/api/incidents",
		"/api/status",
	}
	result := report{Address: address, Status: "pass"}
	for _, path := range paths {
		item, err := probe(client, base, path)
		if err != nil {
			return err
		}
		result.Results = append(result.Results, item)
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func waitReady(ctx context.Context, client *http.Client, url string) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		response, err := client.Get(url)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return errors.New("TideShield did not become ready")
		case <-ticker.C:
		}
	}
}

func probe(client *http.Client, base, path string) (probeResult, error) {
	response, err := client.Get(base + path)
	if err != nil {
		return probeResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return probeResult{}, fmt.Errorf("%s returned HTTP %d", path, response.StatusCode)
	}
	buffer := make([]byte, 1<<20)
	count, err := response.Body.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return probeResult{}, err
	}
	marker := count > 2
	if filepath.Ext(path) == ".html" {
		marker = contains(buffer[:count], []byte("TideShield"))
	}
	if !marker {
		return probeResult{}, fmt.Errorf("%s response marker is missing", path)
	}
	return probeResult{Path: path, Status: response.StatusCode, ContentType: response.Header.Get("Content-Type"), Bytes: response.ContentLength, Marker: marker}, nil
}

func contains(value, marker []byte) bool {
	if len(marker) == 0 {
		return true
	}
	for offset := 0; offset+len(marker) <= len(value); offset++ {
		match := true
		for index := range marker {
			if value[offset+index] != marker[index] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
