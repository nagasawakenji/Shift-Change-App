package handler

import (
	"database/sql"
	"errors"
	"math/rand"
	"net/http"
	"shift-change-app/internal/database"
	"time"

	"github.com/labstack/echo/v4"
)

func (h *Handler) CreateGroup(c echo.Context) error {
	ctx := c.Request().Context()

	// リクエストを受け取る
	type Request struct {
		GroupName string `json:"group_name"`
	}
	var req Request
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.GroupName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	userUUID, err := h.userUUIDFromAuth(c)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not registered"})
		}
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	// 招待コードの作成
	invitationCode := generateRandomString(6)

	tx, err := h.db.Begin()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to start transaction"})
	}
	defer tx.Rollback()

	qtx := h.queries.WithTx(tx)

	// グループの作成
	group, err := qtx.CreateJobGroup(ctx, database.CreateJobGroupParams{
		Name:           req.GroupName,
		InvitationCode: invitationCode,
		OwnerID:        userUUID,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create group: " + err.Error()})
	}

	// 作成したグループに対して、作成者をADMINとしてメンバーに追加する
	_, err = qtx.CreateGroupMember(ctx, database.CreateGroupMemberParams{
		UserID:  userUUID,
		GroupID: group.ID,
		Role:    "ADMIN",
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to add member: " + err.Error()})
	}

	if err := tx.Commit(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to commit transaction"})
	}

	return c.JSON(http.StatusOK, group)
}

// 招待コードを使ってグループに参加
func (h *Handler) JoinGroup(c echo.Context) error {
	ctx := c.Request().Context()

	type Request struct {
		Code string `json:"invitation_code"`
	}
	var req Request
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.Code == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	userUUID, err := h.userUUIDFromAuth(c)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not registered"})
		}
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	// 招待コードからグループを特定
	group, err := h.queries.GetJobGroupByCode(ctx, req.Code)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Invalid invitation code"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// すでにメンバーか確認
	_, err = h.queries.GetGroupMember(ctx, database.GetGroupMemberParams{
		GroupID: group.ID,
		UserID:  userUUID,
	})
	if err == nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "You are already a member of this group"})
	}

	// メンバーに追加 (Role: MEMBER)
	member, err := h.queries.CreateGroupMember(ctx, database.CreateGroupMemberParams{
		UserID:  userUUID,
		GroupID: group.ID,
		Role:    "MEMBER",
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to join group: " + err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Joined successfully",
		"group":   group,
		"member":  member,
	})
}

// ヘルパー関数: ランダムな文字列を生成
func generateRandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
