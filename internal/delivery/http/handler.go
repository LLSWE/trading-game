package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/LLSWE/trading-game/internal/domain"
	"github.com/LLSWE/trading-game/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	wagerUC *usecase.WageringUseCase
}

func NewHandler(wagerUC *usecase.WageringUseCase) *Handler {
	return &Handler{
		wagerUC: wagerUC,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/wallets", func(r chi.Router) {
	})

	r.Route("/wagering", func(r chi.Router) {
		r.Post("/transactions", h.ProcessWagerHandler)
	})
}

func (h *Handler) ProcessWagerHandler(w http.ResponseWriter, r *http.Request) {
	providerID, ok := r.Context().Value(ProviderIDKey).(string)
	if !ok || providerID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized: provider identity missing")
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "missing required header: Idempotency-Key")
		return
	}

	var req usecase.WagerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body format")
		return
	}

	if req.ProviderID != providerID {
		writeError(w, http.StatusForbidden, "forbidden: providerId in body does not match authenticated token")
		return
	}

	if req.IdempotencyKey == "" {
		req.IdempotencyKey = idempotencyKey
	}

	resp, err := h.wagerUC.ProcessTransaction(r.Context(), req)
	if err != nil {
		if errors.Is(err, usecase.ErrInsufficientBalance) {
			writeError(w, http.StatusUnprocessableEntity, "insufficient funds for wager")
			return
		}
		if errors.Is(err, usecase.ErrDuplicateIdempotency) {
			writeError(w, http.StatusConflict, "idempotency key reused with conflicting payload")
			return
		}
		if errors.Is(err, domain.ErrInvalidMoneyFormat) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
