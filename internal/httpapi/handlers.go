package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Mavichy/TestTaskEffectiveMobile/internal/storage"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	log  *slog.Logger
	repo *storage.SubscriptionsRepo
}

func NewHandler(log *slog.Logger, repo *storage.SubscriptionsRepo) *Handler {
	return &Handler{log: log, repo: repo}
}

type createReq struct {
	ServiceName string  `json:"service_name"`
	Price       int     `json:"price"`
	UserID      string  `json:"user_id"`
	StartDate   string  `json:"start_date"`
	EndDate     *string `json:"end_date,omitempty"`
}

type patchReq struct {
	ServiceName *string  `json:"service_name,omitempty"`
	Price       *int     `json:"price,omitempty"`
	UserID      *string  `json:"user_id,omitempty"`
	StartDate   *string  `json:"start_date,omitempty"`
	EndDate     **string `json:"end_date,omitempty"`
}

type subResp struct {
	ID          string  `json:"id"`
	ServiceName string  `json:"service_name"`
	Price       int     `json:"price"`
	UserID      string  `json:"user_id"`
	StartDate   string  `json:"start_date"`
	EndDate     *string `json:"end_date,omitempty"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.ServiceName = strings.TrimSpace(req.ServiceName)
	req.UserID = strings.TrimSpace(req.UserID)

	if req.ServiceName == "" {
		writeErr(w, http.StatusBadRequest, "service_name is required")
		return
	}
	if req.UserID == "" {
		writeErr(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if req.Price <= 0 {
		writeErr(w, http.StatusBadRequest, "price must be > 0")
		return
	}

	start, err := parseMonth(strings.TrimSpace(req.StartDate))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "start_date must be MM-YYYY")
		return
	}

	var end *time.Time
	if req.EndDate != nil {
		s := strings.TrimSpace(*req.EndDate)
		if s == "" {
			writeErr(w, http.StatusBadRequest, "end_date must be MM-YYYY or null")
			return
		}
		t, err := parseMonth(s)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "end_date must be MM-YYYY or null")
			return
		}
		if t.Before(start) {
			writeErr(w, http.StatusBadRequest, "end_date must be >= start_date")
			return
		}
		end = &t
	}

	s, err := h.repo.Create(r.Context(), storage.CreateSubscription{
		ServiceName: req.ServiceName,
		Price:       req.Price,
		UserID:      req.UserID,
		StartDate:   start,
		EndDate:     end,
	})
	if err != nil {
		h.log.Error("create", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, toResp(s))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id is required")
		return
	}

	s, err := h.repo.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		h.log.Error("get", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, toResp(s))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id is required")
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		h.log.Error("delete", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id is required")
		return
	}

	var req patchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}

	var in storage.UpdateSubscription

	if req.ServiceName != nil {
		v := strings.TrimSpace(*req.ServiceName)
		if v == "" {
			writeErr(w, http.StatusBadRequest, "service_name cannot be empty")
			return
		}
		in.ServiceName = &v
	}
	if req.UserID != nil {
		v := strings.TrimSpace(*req.UserID)
		if v == "" {
			writeErr(w, http.StatusBadRequest, "user_id cannot be empty")
			return
		}
		in.UserID = &v
	}
	if req.Price != nil {
		if *req.Price <= 0 {
			writeErr(w, http.StatusBadRequest, "price must be > 0")
			return
		}
		in.Price = req.Price
	}
	if req.StartDate != nil {
		t, err := parseMonth(strings.TrimSpace(*req.StartDate))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "start_date must be MM-YYYY")
			return
		}
		in.StartDate = &t
	}
	if req.EndDate != nil {
		if *req.EndDate == nil {
			var nilTime *time.Time = nil
			in.EndDate = &nilTime
		} else {
			s := strings.TrimSpace(**req.EndDate)
			t, err := parseMonth(s)
			if err != nil {
				writeErr(w, http.StatusBadRequest, "end_date must be MM-YYYY or null")
				return
			}
			tmp := &t
			in.EndDate = &tmp
		}
	}

	s, err := h.repo.Update(r.Context(), id, in)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		h.log.Error("update", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, toResp(s))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	serviceName := strings.TrimSpace(r.URL.Query().Get("service_name"))

	var f storage.ListFilter
	if userID != "" {
		f.UserID = &userID
	}
	if serviceName != "" {
		f.ServiceName = &serviceName
	}
	f.Limit = 50
	f.Offset = 0

	list, err := h.repo.List(r.Context(), f)
	if err != nil {
		h.log.Error("list", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := make([]subResp, 0, len(list))
	for _, s := range list {
		resp = append(resp, toResp(s))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) Total(w http.ResponseWriter, r *http.Request) {
	fromStr := strings.TrimSpace(r.URL.Query().Get("from"))
	toStr := strings.TrimSpace(r.URL.Query().Get("to"))
	if fromStr == "" || toStr == "" {
		writeErr(w, http.StatusBadRequest, "from and to are required (MM-YYYY)")
		return
	}

	from, err := parseMonth(fromStr)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "from must be MM-YYYY")
		return
	}
	to, err := parseMonth(toStr)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "to must be MM-YYYY")
		return
	}
	if from.After(to) {
		writeErr(w, http.StatusBadRequest, "from must be <= to")
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	serviceName := strings.TrimSpace(r.URL.Query().Get("service_name"))
	var pUserID, pServiceName *string
	if userID != "" {
		pUserID = &userID
	}
	if serviceName != "" {
		pServiceName = &serviceName
	}

	total, err := h.repo.TotalCost(r.Context(), from, to, pUserID, pServiceName)
	if err != nil {
		h.log.Error("total", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"total": total})
}

func toResp(s storage.Subscription) subResp {
	var end *string
	if s.EndDate != nil {
		v := formatMonth(*s.EndDate)
		end = &v
	}
	return subResp{
		ID:          s.ID,
		ServiceName: s.ServiceName,
		Price:       s.Price,
		UserID:      s.UserID,
		StartDate:   formatMonth(s.StartDate),
		EndDate:     end,
	}
}
