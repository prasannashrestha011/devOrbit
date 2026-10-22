package traffic_inspector

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const previewLimit = 256 << 10

type FormField struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
}
type Preview struct {
	Text      string      `json:"text"`
	Binary    bool        `json:"binary"`
	Truncated bool        `json:"truncated"`
	Error     string      `json:"error,omitempty"`
	Fields    []FormField `json:"fields,omitempty"`
}
type exchangeDetail struct {
	Exchange
	RequestPreview  Preview `json:"request_preview"`
	ResponsePreview Preview `json:"response_preview"`
}

func bodyPreview(path string, headers http.Header) Preview {
	f, err := os.Open(path)
	if err != nil {
		return Preview{Error: "Captured body is unavailable."}
	}
	defer f.Close()
	var reader io.Reader = f
	encodings := strings.Split(strings.Join(headers.Values("Content-Encoding"), ","), ",")
	for i := len(encodings) - 1; i >= 0; i-- {
		var decoded io.ReadCloser
		switch strings.ToLower(strings.TrimSpace(encodings[i])) {
		case "", "identity":
			continue
		case "gzip", "x-gzip":
			decoded, err = gzip.NewReader(reader)
		case "deflate":
			decoded, err = zlib.NewReader(reader)
		default:
			return Preview{Binary: true, Error: "Preview does not support this content encoding. Download the retained bytes."}
		}
		if err != nil {
			return Preview{Error: "Compressed body could not be decoded. Download the retained bytes."}
		}
		defer decoded.Close()
		reader = decoded
	}
	data, err := io.ReadAll(io.LimitReader(reader, previewLimit+1))
	preview := Preview{Truncated: len(data) > previewLimit}
	data = data[:min(len(data), previewLimit)]
	if err != nil {
		preview.Error = "Body preview is partial: decoding failed."
	}
	contentType, params, _ := mime.ParseMediaType(headers.Get("Content-Type"))
	if contentType == "multipart/form-data" {
		parts := multipart.NewReader(bytes.NewReader(data), params["boundary"])
		for {
			part, err := parts.NextRawPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				preview.Error = "Multipart preview is partial; download the body to inspect all parts."
				break
			}
			value, err := io.ReadAll(part)
			field := FormField{Name: part.FormName(), Filename: part.FileName(), ContentType: part.Header.Get("Content-Type")}
			if field.Filename == "" && utf8.Valid(value) {
				field.Value = string(value)
			} else {
				field.Value = "File/binary part; download the body for its bytes."
			}
			preview.Fields = append(preview.Fields, field)
			if err != nil {
				preview.Error = "Multipart preview is partial; download the body to inspect all parts."
				break
			}
		}
	}
	preview.Binary = !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0
	if preview.Binary {
		preview.Text = hex.Dump(data[:min(len(data), 512)])
		preview.Truncated = preview.Truncated || len(data) > 512
	} else {
		preview.Text = string(data)
	}
	return preview
}

// The session lock keeps Clear/retention from deleting an exchange while it is opened.
func (m *Manager) exchange(workspaceID, sid, id uuid.UUID) (*session, Exchange, error) {
	m.mu.RLock()
	s := m.sessions[sid]
	if s == nil || s.info.Target.WorkspaceID != workspaceID {
		m.mu.RUnlock()
		return nil, Exchange{}, ErrNotFound
	}
	s.mu.Lock()
	m.mu.RUnlock()
	for _, record := range s.records {
		if record.ID == id && timeWithinRetention(record) {
			var exchange Exchange
			err := readCaptureJSON(filepath.Join(m.store.exchangePath(sid, id), "exchange.json"), &exchange)
			if err != nil {
				s.mu.Unlock()
				return nil, Exchange{}, err
			}
			return s, exchange, nil
		}
	}
	s.mu.Unlock()
	return nil, Exchange{}, ErrNotFound
}

func exchangeIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	wid, ok := workspaceID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	sid, ok := sessionID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(chi.URLParam(r, "recordID"))
	if err != nil {
		replyError(w, 400, "invalid_request", "invalid exchange id")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return wid, sid, id, true
}

func (c *Controller) Detail(w http.ResponseWriter, r *http.Request) {
	wid, sid, id, ok := exchangeIDs(w, r)
	if !ok {
		return
	}
	s, exchange, err := c.Manager.exchange(wid, sid, id)
	if err != nil {
		captureError(w, err)
		return
	}
	defer s.mu.Unlock()
	path := c.Manager.store.exchangePath(sid, id)
	replyJSON(w, 200, exchangeDetail{Exchange: exchange,
		RequestPreview:  bodyPreview(filepath.Join(path, "request.body"), exchange.Request.Headers),
		ResponsePreview: bodyPreview(filepath.Join(path, "response.body"), exchange.Response.Headers),
	})
}

func (c *Controller) Body(w http.ResponseWriter, r *http.Request) {
	wid, sid, id, ok := exchangeIDs(w, r)
	if !ok {
		return
	}
	side := chi.URLParam(r, "side")
	if side != "request" && side != "response" {
		replyError(w, 400, "invalid_request", "invalid body side")
		return
	}
	s, _, err := c.Manager.exchange(wid, sid, id)
	if err != nil {
		captureError(w, err)
		return
	}
	f, err := os.Open(filepath.Join(c.Manager.store.exchangePath(sid, id), side+".body"))
	s.mu.Unlock()
	if err != nil {
		captureError(w, err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		captureError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+side+"-"+id.String()+`.bin"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

func captureError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		replyError(w, 404, "not_found", "captured exchange not found")
		return
	}
	replyError(w, 500, "capture_unavailable", "captured data could not be read")
}
