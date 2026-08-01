// Package api serves the mini-acm UI + MockUp REST API.
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/dasmlab/mini-acm/internal/inventory"
	"github.com/dasmlab/mini-acm/internal/mockup"
)

type Server struct {
	store     *mockup.Store
	inventory *inventory.Store
	dataDir   string
	buildVer  string
	static    http.Handler
	router    chi.Router
}

func New(store *mockup.Store, inv *inventory.Store, dataDir, buildVer string, static http.Handler) *Server {
	s := &Server{store: store, inventory: inv, dataDir: dataDir, buildVer: buildVer, static: static}
	s.router = s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return s.router }

func ListenAndServe(addr string, h http.Handler) error {
	return http.ListenAndServe(addr, h)
}

func (s *Server) routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
		MaxAge:         300,
	}))

	r.Get("/healthz", s.healthz)
	r.Get("/isalive", s.healthz)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", s.healthz)
		r.Get("/version", s.version)
		r.Get("/profiles", s.profiles)

		r.Get("/mockups", s.listMockups)
		r.Post("/mockups", s.createMockup)
		r.Get("/mockups/{id}", s.getMockup)
		r.Put("/mockups/{id}", s.putMockup)
		r.Patch("/mockups/{id}/layout", s.patchLayout)
		r.Post("/mockups/{id}/clusters", s.addCluster)
		r.Delete("/mockups/{id}/clusters/{clusterId}", s.deleteCluster)
		r.Post("/mockups/{id}/derive", s.derive)
		r.Post("/mockups/{id}/validate", s.validateMockup)
		r.Delete("/mockups/{id}", s.deleteMockup)

		r.Get("/inventory", s.listInventory)
		r.Post("/inventory", s.createInventory)
		r.Get("/inventory/{id}", s.getInventory)
		r.Put("/inventory/{id}", s.putInventory)
		r.Post("/inventory/{id}/probe", s.probeInventory)
		r.Delete("/inventory/{id}", s.deleteInventory)
	})

	if s.static != nil {
		r.NotFound(s.static.ServeHTTP)
		r.Get("/*", s.static.ServeHTTP)
	}
	return r
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "warning": "LAB/TEST/DEV ONLY", "version": s.buildVer,
	})
}

func (s *Server) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": s.buildVer})
}

func (s *Server) profiles(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"hub": []map[string]any{
			{"id": "hub-supported", "cpu": 8, "memoryMiB": 24576, "diskGiB": 200, "unsupported": false},
			{"id": "hub-lab", "cpu": 8, "memoryMiB": 16384, "diskGiB": 160, "unsupported": true},
		},
		"cluster": []map[string]any{
			{"id": "supported", "cpu": 4, "memoryMiB": 16384, "diskGiB": 120, "unsupported": false},
			{"id": "lab-small", "cpu": 4, "memoryMiB": 12288, "diskGiB": 120, "unsupported": true},
		},
	})
}

func (s *Server) listMockups(w http.ResponseWriter, _ *http.Request) {
	list, err := s.store.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []*mockup.MockUp{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createMockup(w http.ResponseWriter, r *http.Request) {
	var req mockup.CreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	m, err := s.store.Create(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) getMockup(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) putMockup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var m mockup.MockUp
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	m.Metadata.ID = existing.Metadata.ID
	m.Metadata.CreatedAt = existing.Metadata.CreatedAt
	if m.APIVersion == "" {
		m.APIVersion = existing.APIVersion
	}
	if m.Kind == "" {
		m.Kind = "MockUp"
	}
	if err := s.store.Save(&m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, &m)
}

func (s *Server) patchLayout(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var layout mockup.Layout
	if err := json.NewDecoder(r.Body).Decode(&layout); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	m.Layout = layout
	if err := s.store.Save(m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) addCluster(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	c := m.AddCluster()
	if err := s.store.Save(m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"cluster": c, "mockup": m})
}

func (s *Server) deleteCluster(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	clusterID := chi.URLParam(r, "clusterId")
	m, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := m.RemoveCluster(clusterID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.store.Save(m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) derive(w http.ResponseWriter, r *http.Request) {
	paths, err := s.store.Derive(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"paths": paths})
}

func (s *Server) validateMockup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var m *mockup.MockUp
	if r.Body != nil && r.ContentLength != 0 {
		var body mockup.MockUp
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		body.Metadata.ID = id
		m = &body
	} else {
		got, err := s.store.Get(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		m = got
	}
	writeJSON(w, http.StatusOK, mockup.ValidateTopology(m))
}

func (s *Server) deleteMockup(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Delete(chi.URLParam(r, "id")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listInventory(w http.ResponseWriter, _ *http.Request) {
	if s.inventory == nil {
		writeJSON(w, http.StatusOK, []*inventory.MachineHost{})
		return
	}
	list, err := s.inventory.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []*inventory.MachineHost{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createInventory(w http.ResponseWriter, r *http.Request) {
	if s.inventory == nil {
		http.Error(w, "inventory not configured", http.StatusServiceUnavailable)
		return
	}
	var req inventory.CreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h, err := s.inventory.Create(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, h)
}

func (s *Server) getInventory(w http.ResponseWriter, r *http.Request) {
	if s.inventory == nil {
		http.Error(w, "inventory not configured", http.StatusServiceUnavailable)
		return
	}
	h, err := s.inventory.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, h)
}

func (s *Server) putInventory(w http.ResponseWriter, r *http.Request) {
	if s.inventory == nil {
		http.Error(w, "inventory not configured", http.StatusServiceUnavailable)
		return
	}
	id := chi.URLParam(r, "id")
	existing, err := s.inventory.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var h inventory.MachineHost
	if err := json.NewDecoder(r.Body).Decode(&h); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.ID = existing.ID
	h.CreatedAt = existing.CreatedAt
	h.Seed = existing.Seed || h.Seed
	if err := s.inventory.Save(&h); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, &h)
}

func (s *Server) probeInventory(w http.ResponseWriter, r *http.Request) {
	if s.inventory == nil {
		http.Error(w, "inventory not configured", http.StatusServiceUnavailable)
		return
	}
	res, err := s.inventory.Probe(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) deleteInventory(w http.ResponseWriter, r *http.Request) {
	if s.inventory == nil {
		http.Error(w, "inventory not configured", http.StatusServiceUnavailable)
		return
	}
	if err := s.inventory.Delete(chi.URLParam(r, "id")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// StaticFS serves an embedded SPA (interview-me / etcd-synthetic-load style).
type StaticFS struct {
	Root http.FileSystem
}

func (s StaticFS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.Root == nil {
		http.NotFound(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	f, err := s.Root.Open(path)
	if err != nil || isDir(f) {
		if f != nil {
			_ = f.Close()
		}
		f, err = s.Root.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		path = "index.html"
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "static file not seekable", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, path, stat.ModTime(), rs)
}

func isDir(f http.File) bool {
	if f == nil {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.IsDir()
}
