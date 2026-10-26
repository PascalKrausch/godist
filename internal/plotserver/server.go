package plotserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/PascalKrausch/godist/internal/plot"
)

// Source liefert die Diagrammdaten für eine Klassenanzahl. bins = 0 bedeutet automatische Wahl.
type Source func(bins int) plot.Payload

// Handler liefert das Dashboard und die JSON-API. addr ist die Adresse, auf der der Server lauscht
// (z. B. "127.0.0.1:8080"). Nur Anfragen mit genau diesem Host werden beantwortet; das verhindert
// DNS-Rebinding-Zugriffe von fremden Webseiten auf die lokalen Daten.
// GET /api/plot?bins=N berechnet die Daten mit N Klassen; ohne Parameter wird die Klassenanzahl automatisch gewählt.
func Handler(source Source, addr string) (http.Handler, error) {
	assets, err := webAssets()
	if err != nil {
		return nil, fmt.Errorf("load web assets: %w", err)
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("parse address %q: %w", addr, err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/plot", func(w http.ResponseWriter, r *http.Request) {
		bins, err := parseBins(r.URL.Query().Get("bins"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")

		if err := json.NewEncoder(w).Encode(source(bins)); err != nil {
			// Die Antwort könnte bereits teilweise geschrieben sein.
			// Nicht versuchen, danach noch eine zweite JSON-Antwort zu senden.
			return
		}
	})

	mux.Handle("GET /", http.FileServer(http.FS(assets)))

	allowedHosts := map[string]bool{
		"127.0.0.1:" + port: true,
		"localhost:" + port: true,
	}
	return rejectForeignHosts(allowedHosts, mux), nil
}

// parseBins liest den Query-Parameter bins. Leer bedeutet automatisch (0).
func parseBins(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	bins, err := strconv.Atoi(raw)
	if err != nil || bins < 1 || bins > plot.MaxBins {
		return 0, fmt.Errorf("bins muss eine ganze Zahl zwischen 1 und %d sein", plot.MaxBins)
	}
	return bins, nil
}

func rejectForeignHosts(allowedHosts map[string]bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedHosts[r.Host] {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Serve startet das Dashboard auf einem freien lokalen Port und blockiert, bis ctx beendet wird.
// onReady wird mit der URL aufgerufen, sobald der Port gebunden ist (darf nil sein).
func Serve(ctx context.Context, source Source, onReady func(url string)) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen on localhost: %w", err)
	}

	handler, err := Handler(source, listener.Addr().String())
	if err != nil {
		_ = listener.Close()
		return err
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if onReady != nil {
		onReady("http://" + listener.Addr().String())
	}

	// Die Goroutine endet mit dem Shutdown oder mit dem Ende von Serve, damit nichts hängen bleibt.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(
				context.Background(), 5*time.Second,
			)
			defer cancel()
			_ = server.Shutdown(shutdownCtx)
		case <-done:
		}
	}()

	err = server.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	if err != nil {
		return fmt.Errorf("serve dashboard: %w", err)
	}
	return nil
}
