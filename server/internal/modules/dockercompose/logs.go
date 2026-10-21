package dockercompose

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

type logFrame struct {
	Stream string `json:"stream"`
	Data   string `json:"data"`
}

func (c *Controller) Logs(w http.ResponseWriter, r *http.Request) {
	workspace, ok := c.workspace(w, r)
	if !ok {
		return
	}
	root, err := filepath.EvalSymlinks(workspace.RootPath)
	if err != nil {
		replyError(w, 400, "invalid_workspace", err.Error())
		return
	}
	if available := c.Manager.Availability(r.Context(), root); available.State != "ready" {
		replyError(w, 503, available.State, available.Message)
		return
	}
	_, project, err := c.prepare(r.Context(), root, r.URL.Query().Get("file"))
	if err != nil {
		replyError(w, 400, "invalid_compose", err.Error())
		return
	}
	if err := c.Manager.Status(r.Context(), project.Directory, project); err != nil {
		replyError(w, 409, "project_unavailable", err.Error())
		return
	}
	serviceName := chi.URLParam(r, "service")
	found := false
	for _, service := range project.Services {
		if service.Name == serviceName && len(service.Containers) > 0 {
			found = true
			break
		}
	}
	if !found || strings.HasPrefix(serviceName, "-") {
		replyError(w, 404, "not_found", "Compose service has no containers")
		return
	}
	tail := 200
	if raw := r.URL.Query().Get("tail"); raw != "" {
		tail, err = strconv.Atoi(raw)
		if err != nil || tail < 1 || tail > 2000 {
			replyError(w, 400, "invalid_request", "tail must be between 1 and 2000")
			return
		}
	}
	args := append(composeArgs(project.Directory, Candidate{Files: project.Files}), "logs", "--timestamps", "--no-color", "--tail", strconv.Itoa(tail), "--follow", serviceName)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = project.Directory
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		replyError(w, 500, "logs_unavailable", "Cannot open Docker logs")
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		replyError(w, 500, "logs_unavailable", "Cannot open Docker logs")
		return
	}
	if err := cmd.Start(); err != nil {
		replyError(w, 503, "logs_unavailable", err.Error())
		return
	}
	frames := make(chan logFrame, 128)
	var readers sync.WaitGroup
	for _, source := range []struct {
		name string
		pipe io.Reader
	}{{"stdout", stdout}, {"stderr", stderr}} {
		readers.Add(1)
		go func() {
			defer readers.Done()
			scanner := bufio.NewScanner(source.pipe)
			scanner.Buffer(make([]byte, 64*1024), 1<<20)
			for scanner.Scan() {
				select {
				case frames <- logFrame{source.name, scanner.Text()}:
				case <-ctx.Done():
					return
				}
			}
			if scanner.Err() != nil {
				select {
				case frames <- logFrame{"error", "Docker log line exceeded 1 MiB or the stream failed."}:
				case <-ctx.Done():
				}
			}
		}()
	}
	go func() { readers.Wait(); close(frames) }()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		cancel()
		_ = cmd.Wait()
		return
	}
	heartbeat := time.NewTicker(5 * time.Second)
	defer heartbeat.Stop()
	defer func() { cancel(); _ = cmd.Wait() }()
	for {
		select {
		case frame, open := <-frames:
			if !open {
				return
			}
			body, _ := json.Marshal(frame)
			_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := fmt.Fprintf(w, "data: %s\n\n", body); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
