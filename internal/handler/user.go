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

// 退会処理
func (h *Handler) WithdrawMe(c echo.Context) error {
	ctx := c.Request().Context()

	sub, ok := LineSub(c)
	if !ok || sub == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	user, err := h.queries.GetUserByLineID(ctx, sub)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get user"})
	}

	// 複数処理があるので、Transaction
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to begin tx"})
	}
	defer tx.Rollback()

	qtx := h.queries.WithTx(tx)

	// 退会ユーザーの OPEN 募集を全部 CLOSED にする
	_, err = qtx.CloseOpenShiftTradesByRequester(ctx, user.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to close open trades"})
	}

	// users を匿名化して deleted_at を立てる
	deletedLineID := "deleted:" + user.ID.String()
	displayName := "退会ユーザー"

	err = qtx.WithdrawUser(ctx, database.WithdrawUserParams{
		ID:          user.ID,
		LineUserID:  deletedLineID,
		DisplayName: displayName,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to withdraw user"})
	}

	if err := tx.Commit(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to commit tx"})
	}

	return c.NoContent(http.StatusOK)
}
