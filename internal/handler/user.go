package handler

import (
	"database/sql"
	"net/http"
	"shift-change-app/internal/database"

	"github.com/labstack/echo/v4"
)

// ユーザー登録
func (h *Handler) RegisterUser(c echo.Context) error {
	ctx := c.Request().Context()

	type Request struct {
		Name string `json:"name"`
	}
	var req Request
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Request"})
	}
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Request"})
	}

	sub, ok := LineSub(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	// 登録
	user, err := h.queries.CreateUser(ctx, database.CreateUserParams{
		LineUserID:      sub,
		DisplayName:     req.Name,
		ProfileImageUrl: sql.NullString{Valid: false},
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, user)
}

// ユーザー取得
func (h *Handler) GetUser(c echo.Context) error {
	lineID := c.Param("line_id")
	ctx := c.Request().Context()

	user, err := h.queries.GetUserByLineID(ctx, lineID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, user)
}
