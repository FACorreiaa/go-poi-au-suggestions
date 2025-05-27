package userInterest

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

// UserInterestHandler handles HTTP requests related to user operations.
type UserInterestHandler struct {
	userInterestService UserInterestService
	logger              *slog.Logger
}

// NewUserInterestHandler creates a new user handler instance.
func NewUserInterestHandler(userInterestService UserInterestService, logger *slog.Logger) *UserInterestHandler {
	instanceAddress := fmt.Sprintf("%p", logger)
	slog.Info("Creating NewUserInterestHandler", slog.String("logger_address", instanceAddress), slog.Bool("logger_is_nil", logger == nil))
	if logger == nil {
		panic("PANIC: Attempting to create UserInterestHandler with nil logger!")
	}

	return &UserInterestHandler{
		userInterestService: userInterestService,
		logger:              logger,
	}
}

// GetAllInterests godoc
// @Summary      Get All Interests
// @Description  Retrieves all available interests.
// @Tags         User
// @Accept       json
// @Produce      json
// @Success      200 {array} types.Interest "All Interests"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Router       /user/interests [get]
func (h *UserInterestHandler) GetAllInterests(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserInterestHandler").Start(r.Context(), "GetAllInterests", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/interests"),
	))
	defer span.End()

	l := h.logger.With(slog.String("handler", "GetAllInterests"))

	interests, err := h.userInterestService.GetAllInterests(ctx)
	if err != nil {
		l.ErrorContext(ctx, "Failed to get all interests", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get all interests")
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to retrieve interests")
		return
	}

	span.SetStatus(codes.Ok, "All interests retrieved successfully")
	api.WriteJSONResponse(w, r, http.StatusOK, interests)
}

// RemoveUserInterest godoc
// @Summary      Remove User Interest
// @Description  Removes an interest from the authenticated user's preferences.
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        interestId path string true "Interest ID to remove"
// @Success      200 {object} types.Response "Interest Removed Successfully"
// @Failure      400 {object} types.Response "Invalid Input"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "Interest Not Found"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/preferences/interests/{interestId} [delete]
func (h *UserInterestHandler) RemoveUserInterest(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserInterestHandler").Start(r.Context(), "RemoveUserInterest", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/preferences/interests/{interestId}"),
	))
	defer span.End()

	l := h.logger.With(slog.String("handler", "RemoveUserInterest"))

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "User ID not found in context")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid user ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	interestIDStr := chi.URLParam(r, "interestID")
	interestID, err := uuid.Parse(interestIDStr)
	if err != nil {
		l.WarnContext(ctx, "Invalid interest ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid interest ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid interest ID format")
		return
	}

	err = h.userInterestService.RemoveUserInterest(ctx, userID, interestID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to remove user interest", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to remove user interest")
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusNotFound, "Interest association not found")
		} else {
			api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to remove interest")
		}
		return
	}

	span.SetStatus(codes.Ok, "User interest removed successfully")
	api.WriteJSONResponse(w, r, http.StatusOK, types.Response{
		Success: true,
		Message: "Interest removed successfully",
	})
}

// CreateInterest godoc
// @Summary      Create New Interest
// @Description  Creates a new interest in the system.
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        interest body types.CreateInterestRequest true "Interest details to create"
// @Success      201 {object} types.Interest "Created Interest"
// @Failure      400 {object} types.Response "Invalid Input"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      409 {object} types.Response "Interest Already Exists"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/interests/create [post]
func (h *UserInterestHandler) CreateInterest(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserInterestHandler").Start(r.Context(), "CreateInterest", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/interests/create"),
	))
	defer span.End()

	l := h.logger.With(slog.String("handler", "CreateInterest"))

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "User ID not found in context")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Parse request body
	var req types.CreateInterestRequest
	if err := api.DecodeJSONBody(w, r, &req); err != nil {
		l.WarnContext(ctx, "Failed to decode request", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to decode request")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Validate request
	if req.Name == "" {
		l.WarnContext(ctx, "Interest name is required")
		span.SetStatus(codes.Error, "Interest name is required")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Interest name is required")
		return
	}

	// Call service to create interest
	interest, err := h.userInterestService.CreateInterest(ctx, req.Name, req.Description, req.Active, userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Failed to create interest", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create interest")

		if errors.Is(err, types.ErrConflict) {
			api.ErrorResponse(w, r, http.StatusConflict, "Interest with this name already exists")
		} else {
			api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to create interest")
		}
		return
	}

	span.SetStatus(codes.Ok, "Interest created successfully")
	api.WriteJSONResponse(w, r, http.StatusCreated, interest)
}

// GetUserEnhancedInterests godoc
// @Summary      Get User Enhanced Interests
// @Description  Retrieves the authenticated user's enhanced interests with preference levels.
// @Tags         User
// @Accept       json
// @Produce      json
// @Success      200 {array} types.EnhancedInterest "User Enhanced Interests"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/preferences/enhanced [get]
//func (h *UserInterestHandler) GetUserEnhancedInterests(w http.ResponseWriter, r *http.Request) {
//	ctx, span := otel.Tracer("UserInterestHandler").Start(r.Context(), "GetUserEnhancedInterests", trace.WithAttributes(
//		semconv.HTTPRequestMethodKey.String(r.Method),
//		semconv.HTTPRouteKey.String("/user/preferences/enhanced"),
//	))
//	defer span.End()
//
//	l := h.logger.With(slog.String("handler", "GetUserEnhancedInterests"))
//
//	// Get UserID from context (set by Authenticate middleware)
//	userIDStr, ok := api.GetUserIDFromContext(ctx)
//	if !ok || userIDStr == "" {
//		l.ErrorContext(ctx, "User ID not found in context")
//		span.SetStatus(codes.Error, "User ID not found in context")
//		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
//		return
//	}
//
//	userID, err := uuid.Parse(userIDStr)
//	if err != nil {
//		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
//		span.RecordError(err)
//		span.SetStatus(codes.Error, "Invalid user ID format")
//		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
//		return
//	}
//
//	interests, err := h.userInterestService.GetUserEnhancedInterests(ctx, userID)
//	if err != nil {
//		l.ErrorContext(ctx, "Failed to get user enhanced interests", slog.Any("error", err))
//		span.RecordError(err)
//		span.SetStatus(codes.Error, "Failed to get user enhanced interests")
//		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to retrieve enhanced interests")
//		return
//	}
//
//	span.SetStatus(codes.Ok, "User enhanced interests retrieved successfully")
//	api.WriteJSONResponse(w, r, http.StatusOK, interests)
//}

// UpdateUserInterest godoc
// @Summary      Update Custom Interest
// @Description  Updates a specific interest created by the authenticated user.
// @Tags         User Preferences
// @Accept       json
// @Produce      json
// @Param        interestID path string true "ID of the custom interest to update" Format(uuid)
// @Param        interest body types.UpdateUserInterestParams true "Fields to update"
// @Success      200 {object} types.Response "Interest Updated Successfully"
// @Failure      400 {object} types.Response "Invalid Input or Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      403 {object} types.Response "Forbidden (Interest does not belong to user)"
// @Failure      404 {object} types.Response "Interest Not Found"
// @Failure      409 {object} types.Response "Conflict (e.g., duplicate name for user)"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/custom-interests/{interestID} [put] // Changed route for clarity
func (h *UserInterestHandler) UpdateUserInterest(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserHandler").Start(r.Context(), "UpdateCustomInterest", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		// Note: Getting the exact route template might require specific Chi helpers if available
		// semconv.HTTPRouteKey.String("/user/custom-interests/{interestID}"),
	))
	defer span.End()

	l := h.logger.With(slog.String("handler", "UpdateCustomInterest"))

	// 1. Get UserID from context
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.WarnContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "Authentication required")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil { // Should ideally not happen if JWT sub is always valid UUID
		l.ErrorContext(ctx, "Invalid user ID format in context", slog.String("user_id_str", userIDStr), slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid User ID in token")
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Invalid user session")
		return
	}
	l = l.With(slog.String("userID", userID.String())) // Add userID to logger
	span.SetAttributes(attribute.String("user.id", userID.String()))

	// 2. Get profileID from path parameter
	interestIDStr := chi.URLParam(r, "interestID")
	interestID, err := uuid.Parse(interestIDStr)
	if err != nil {
		l.WarnContext(ctx, "Invalid interest ID format in URL path", slog.String("interest_id_str", interestIDStr), slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid interest ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid interest ID format in URL")
		return
	}
	l = l.With(slog.String("interestID", interestID.String())) // Add interestID to logger
	span.SetAttributes(attribute.String("interest.id", interestID.String()))

	// 3. Decode request body
	var params types.UpdateUserInterestParams
	if err := api.DecodeJSONBody(w, r, &params); err != nil {
		l.WarnContext(ctx, "Failed to decode request body", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid request body")
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid request format: %s", err.Error()))
		return
	}

	// Basic validation: Ensure at least one field is being updated
	if params.Name == nil && params.Description == nil && params.Active == nil {
		l.WarnContext(ctx, "Update request received with no fields to update")
		span.SetStatus(codes.Error, "No update fields provided")
		api.ErrorResponse(w, r, http.StatusBadRequest, "No fields provided for update")
		return
	}

	// 4. Call service
	err = h.userInterestService.UpdateUserInterest(ctx, userID, interestID, params)
	if err != nil {
		l.ErrorContext(ctx, "Service failed to update custom interest", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Service update failed")

		// Map domain errors to HTTP status codes
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusNotFound, "Custom interest not found or does not belong to user")
		} else if errors.Is(err, types.ErrConflict) {
			api.ErrorResponse(w, r, http.StatusConflict, "Custom interest with this name already exists")
		} else if errors.Is(err, types.ErrBadRequest) {
			api.ErrorResponse(w, r, http.StatusBadRequest, err.Error()) // Use specific message from error
		} else {
			api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to update custom interest")
		}
		return
	}

	// 5. Respond successfully
	l.InfoContext(ctx, "Custom interest updated successfully")
	span.SetStatus(codes.Ok, "Custom interest updated")
	api.WriteJSONResponse(w, r, http.StatusOK, types.Response{Success: true, Message: "Interest updated successfully"})
}
