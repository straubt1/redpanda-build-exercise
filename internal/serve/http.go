package serve

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/straubt1/redpanda-build-exercise/internal/applog"
	"github.com/straubt1/redpanda-build-exercise/internal/store"
)

var errBadSort = errors.New("unknown sort column")

type Server struct {
	db   *store.Store
	page *template.Template
}

func New(db *store.Store) *Server {
	return &Server{
		db: db,
		page: template.Must(template.New("page").Funcs(template.FuncMap{
			"fmtTime": fmtTime,
			"fmtConf": fmtConf,
			"nextDir": nextDir,
		}).Parse(pageHTML)),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /api/triages", s.apiTriages)
	mux.HandleFunc("GET /{$}", s.index)
	return mux
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(r.Context()); err != nil {
		http.Error(w, "unhealthy", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	rows, sort, dir, err := s.load(r.Context(), r, false)
	if err != nil {
		applog.Err.Printf("serve list: %v", err)
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.page.Execute(w, pageData{Rows: rows, Sort: sort, Dir: dir}); err != nil {
		applog.Err.Printf("serve template: %v", err)
	}
}

func (s *Server) apiTriages(w http.ResponseWriter, r *http.Request) {
	rows, sort, dir, err := s.load(r.Context(), r, true)
	if err != nil {
		if errors.Is(err, errBadSort) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": errBadSort.Error()})
			return
		}
		applog.Err.Printf("serve list: %v", err)
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(apiResponse{Rows: rows, Sort: sort, Dir: dir})
}

func (s *Server) load(ctx context.Context, r *http.Request, strict bool) ([]store.Row, string, string, error) {
	qSort := r.URL.Query().Get("sort")
	qDir := r.URL.Query().Get("dir")
	col, dir, ok := store.NormalizeOrder(qSort, qDir)
	if !ok && strict && qSort != "" {
		return nil, "", "", errBadSort
	}
	if !ok {
		col, dir = store.DefaultSort, store.DefaultDir
	}
	rows, err := s.db.List(ctx, col, dir)
	if err != nil {
		return nil, "", "", err
	}
	if rows == nil {
		rows = []store.Row{}
	}
	return rows, col, dir, nil
}

type pageData struct {
	Rows []store.Row
	Sort string
	Dir  string
}

type apiResponse struct {
	Rows []store.Row `json:"rows"`
	Sort string      `json:"sort"`
	Dir  string      `json:"dir"`
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04")
}

func fmtConf(c *float64) string {
	if c == nil {
		return ""
	}
	return strconv.FormatFloat(*c, 'f', -1, 64)
}

func nextDir(currentSort, currentDir, col string) string {
	if currentSort == col && currentDir == "desc" {
		return "asc"
	}
	return "desc"
}
