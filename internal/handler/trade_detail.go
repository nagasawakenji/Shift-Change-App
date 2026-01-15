package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"shift-change-app/internal/database"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// シフト交換リクエストの詳細表示
func (h *Handler) ShowTradeDetail(c echo.Context) error {
	ctx := c.Request().Context()

	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid group_id")
	}
	tradeID, err := uuid.Parse(c.Param("trade_id"))
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid trade_id")
	}

	// user_id は表示のためだけに持ってる
	userIDStr := c.QueryParam("user_id")

	// データ取得
	trade, err := h.queries.GetTradeByID(ctx, tradeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.String(http.StatusNotFound, "Trade not found")
		}
		return c.String(http.StatusInternalServerError, "Failed to fetch trade")
	}
	if trade.GroupID != groupID {
		return c.String(http.StatusNotFound, "Trade not found")
	}

	group, err := h.queries.GetJobGroupByID(ctx, groupID)
	if err != nil {
		return c.String(http.StatusNotFound, "Group not found")
	}

	requester, _ := h.queries.GetUserByID(ctx, trade.RequesterID)

	canEdit := false
	if userIDStr != "" {
		if userUUID, err := uuid.Parse(userIDStr); err == nil {
			canEdit = (trade.RequesterID == userUUID)
		}
	}

	data := map[string]interface{}{
		"Group":          group,
		"Trade":          trade,
		"Requester":      requester,
		"CurrentUserID":  userIDStr,
		"GroupID":        groupID.String(),
		"CanEditDetails": canEdit,
		"LiffID":         os.Getenv("LIFF_ID"),
	}
	return c.Render(http.StatusOK, "trade_detail.html", data)
}

// シフト交換リクエストの詳細編集
func (h *Handler) UpdateTradeDetails(c echo.Context) error {
	ctx := c.Request().Context()

	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid group_id"})
	}
	tradeID, err := uuid.Parse(c.Param("trade_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid trade_id"})
	}

	type Req struct {
		Details string `json:"details"`
	}
	var req Req
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	userUUID, err := h.userUUIDFromAuth(c)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not registered"})
		}
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	// trade 取得して group を一致確認
	trade, err := h.queries.GetTradeByID(ctx, tradeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Trade not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if trade.GroupID != groupID {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Trade not found"})
	}

	// 所属チェック
	if _, err := h.queries.GetGroupMember(ctx, database.GetGroupMemberParams{
		GroupID: groupID,
		UserID:  userUUID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You are not a member"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// 作成者だけ更新可能（SQLで requester_id を条件にしてる）
	updated, err := h.queries.UpdateTradeDetails(ctx, database.UpdateTradeDetailsParams{
		ID:          tradeID,
		RequesterID: userUUID,
		Details:     req.Details,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Only requester can update details"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, updated)
}
