package grappadcim

import (
	"database/sql"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const syntheticRingKMLArtifactBase = 900000000

func (h *Handler) handleGetFiberRingKML(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	ringID, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_fiber_ring_id")
		return
	}
	ring, found, err := h.getFiberRing(r.Context(), ringID)
	if err != nil {
		h.dbFailure(w, r, "get_fiber_ring_kml_ring", err, "ring_id", ringID)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "fiber_ring_not_found")
		return
	}
	artifacts, err := h.fiberRingArtifacts(r, ring)
	if err != nil {
		h.dbFailure(w, r, "get_fiber_ring_kml", err, "ring_id", ringID)
		return
	}
	httputil.JSON(w, http.StatusOK, FiberRingKML{RingID: ringID, Artifacts: artifacts})
}

func (h *Handler) handleUploadFiberRingKML(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	ringID, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_fiber_ring_id")
		return
	}
	ring, found, err := h.getFiberRing(r.Context(), ringID)
	if err != nil {
		h.dbFailure(w, r, "upload_fiber_ring_kml_ring", err, "ring_id", ringID)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "fiber_ring_not_found")
		return
	}
	if h.artifactRoot == "" {
		httputil.Error(w, http.StatusServiceUnavailable, "grappa_dcim_artifact_root_not_configured")
		return
	}
	if err := r.ParseMultipartForm(24 << 20); err != nil {
		invalidRequest(w, "invalid_kml_upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		invalidRequest(w, "kml_file_required")
		return
	}
	defer file.Close()
	fileName := safeArtifactFileName(header.Filename)
	if fileName == "" {
		invalidRequest(w, "kml_file_required")
		return
	}
	if ext := strings.ToLower(filepath.Ext(fileName)); ext != ".kml" && ext != ".kmz" {
		invalidRequest(w, "kml_file_type_required")
		return
	}
	storageKeyDir := filepath.ToSlash(filepath.Join("kml", fmt.Sprintf("ring-%d", ringID)))
	storageDir := filepath.Join(h.artifactRoot, filepath.FromSlash(storageKeyDir))
	if err := os.MkdirAll(storageDir, 0o700); err != nil {
		h.dbFailure(w, r, "upload_fiber_ring_kml_mkdir", err, "ring_id", ringID)
		return
	}
	storedName := fmt.Sprintf("%d-%s", time.Now().UnixNano(), fileName)
	storedKey := filepath.ToSlash(filepath.Join(storageKeyDir, storedName))
	storedPath := filepath.Join(storageDir, storedName)
	dst, err := os.OpenFile(storedPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		h.dbFailure(w, r, "upload_fiber_ring_kml_open", err, "ring_id", ringID)
		return
	}
	if _, err := io.Copy(dst, file); err != nil {
		_ = dst.Close()
		h.dbFailure(w, r, "upload_fiber_ring_kml_copy", err, "ring_id", ringID)
		return
	}
	if err := dst.Close(); err != nil {
		h.dbFailure(w, r, "upload_fiber_ring_kml_close", err, "ring_id", ringID)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = fileName
	}
	detail := strings.TrimSpace(r.FormValue("detail"))
	var artifactID int64
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `UPDATE anelli_fibra SET kml_file_path = ? WHERE id_anello = ?`, storedKey, ringID); err != nil {
			return err
		}
		result, err := tx.ExecContext(r.Context(), `
			INSERT INTO mappa_tracciati_anelli (nome, kml, nome_anello, dettagli_tracciato)
			VALUES (?, ?, ?, ?)`, name, storedKey, ring.Name, nullableUploadDetail(detail))
		if err != nil {
			return err
		}
		artifactID, _ = result.LastInsertId()
		return nil
	}); err != nil {
		_ = os.Remove(storedPath)
		h.dbFailure(w, r, "upload_fiber_ring_kml", err, "ring_id", ringID)
		return
	}
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: int(artifactID), Message: "KML caricato."})
}

func (h *Handler) handleDownloadArtifact(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	artifactID, err := parsePathInt(r, "artifactId")
	if err != nil {
		invalidRequest(w, "invalid_artifact_id")
		return
	}
	path, name, found, err := h.artifactPath(r, artifactID)
	if err != nil {
		h.dbFailure(w, r, "download_artifact_lookup", err, "artifact_id", artifactID)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "artifact_not_found")
		return
	}
	cleanPath, ok := h.resolveArtifactPath(path)
	if !ok {
		httputil.Error(w, http.StatusNotFound, "artifact_unavailable")
		return
	}
	stat, err := os.Stat(cleanPath)
	if err != nil || stat.IsDir() {
		httputil.Error(w, http.StatusNotFound, "artifact_unavailable")
		return
	}
	file, err := os.Open(cleanPath)
	if err != nil {
		httputil.Error(w, http.StatusNotFound, "artifact_unavailable")
		return
	}
	defer file.Close()
	fileName := safeArtifactFileName(name)
	if fileName == "" {
		fileName = filepath.Base(cleanPath)
	}
	if contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(fileName))); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "application/vnd.google-earth.kml+xml")
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	http.ServeContent(w, r, fileName, stat.ModTime(), file)
}

func (h *Handler) fiberRingArtifacts(r *http.Request, ring FiberRing) ([]Artifact, error) {
	items := []Artifact{}
	var ringPath sql.NullString
	if err := h.grappa.QueryRowContext(r.Context(), `SELECT kml_file_path FROM anelli_fibra WHERE id_anello = ?`, ring.ID).Scan(&ringPath); err != nil {
		return items, err
	}
	if path := nullableString(ringPath); path != nil {
		items = append(items, h.artifactFromPath(syntheticRingKMLArtifactBase+ring.ID, "KML anello", *path, &ring.Name, nil))
	}
	rows, err := h.grappa.QueryContext(r.Context(), `
		SELECT id, nome, kml, nome_anello, dettagli_tracciato
		FROM mappa_tracciati_anelli m
		WHERE `+fiberRingKMLAssociationByNameSQL("m")+`
		ORDER BY id DESC`, ring.Name, ring.Name)
	if err != nil {
		return items, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name, kml string
		var ringName, detail sql.NullString
		if err := rows.Scan(&id, &name, &kml, &ringName, &detail); err != nil {
			return items, err
		}
		items = append(items, h.artifactFromPath(id, name, kml, nullableString(ringName), nullableString(detail)))
	}
	return items, rows.Err()
}

func (h *Handler) artifactFromPath(id int, name string, path string, ringName *string, detail *string) Artifact {
	fileName := safeArtifactFileName(filepath.Base(path))
	if fileName == "." || fileName == string(filepath.Separator) {
		fileName = ""
	}
	available := false
	if resolved, ok := h.resolveArtifactPath(path); ok {
		if stat, err := os.Stat(resolved); err == nil && !stat.IsDir() {
			available = true
		}
	}
	var downloadURL *string
	if available {
		url := fmt.Sprintf("/api/grappa-dcim/v1/artifacts/%d/download", id)
		downloadURL = &url
	}
	return Artifact{
		ID:          id,
		Kind:        "kml",
		Name:        strings.TrimSpace(name),
		FileName:    fileName,
		RingName:    ringName,
		Detail:      detail,
		Available:   available,
		DownloadURL: downloadURL,
	}
}

func (h *Handler) artifactPath(r *http.Request, artifactID int) (string, string, bool, error) {
	if artifactID > syntheticRingKMLArtifactBase {
		ringID := artifactID - syntheticRingKMLArtifactBase
		var path sql.NullString
		var name string
		err := h.grappa.QueryRowContext(r.Context(), `SELECT kml_file_path, nome FROM anelli_fibra WHERE id_anello = ?`, ringID).Scan(&path, &name)
		if err == sql.ErrNoRows {
			return "", "", false, nil
		}
		if err != nil {
			return "", "", false, err
		}
		if value := nullableString(path); value != nil {
			return *value, filepath.Base(*value), true, nil
		}
		return "", name, false, nil
	}
	var path, name string
	err := h.grappa.QueryRowContext(r.Context(), `SELECT kml, nome FROM mappa_tracciati_anelli WHERE id = ?`, artifactID).Scan(&path, &name)
	if err == sql.ErrNoRows {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return path, name, true, nil
}

func cleanArtifactRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	if abs, err := filepath.Abs(root); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(root)
}

func (h *Handler) resolveArtifactPath(stored string) (string, bool) {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return "", false
	}
	if filepath.IsAbs(stored) {
		return filepath.Clean(stored), true
	}
	if h.artifactRoot == "" {
		return "", false
	}
	rel := filepath.Clean(filepath.FromSlash(stored))
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	resolved := filepath.Join(h.artifactRoot, rel)
	insideRoot, err := filepath.Rel(h.artifactRoot, resolved)
	if err != nil || insideRoot == ".." || strings.HasPrefix(insideRoot, ".."+string(filepath.Separator)) {
		return "", false
	}
	return resolved, true
}

func safeArtifactFileName(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	name = strings.ReplaceAll(name, string(filepath.Separator), "")
	return name
}

func nullableUploadDetail(value string) any {
	if value == "" {
		return nil
	}
	return value
}
