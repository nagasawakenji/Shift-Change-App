package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (h *Handler) ShowHomeEntry(c echo.Context) error {
	return c.Render(http.StatusOK, "home_entry.html", nil)
}

// ホーム画面 (グループ選択)
func (h *Handler) ShowHome(c echo.Context) error {
	ctx := c.Request().Context()
	userIDStr := c.QueryParam("user_id")

	if userIDStr == "" {
		return c.String(http.StatusBadRequest, "user_id is required")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid user_id")
	}

	user, err := h.queries.GetUserByID(ctx, userID)
	if err != nil {
		return c.String(http.StatusNotFound, "User not found")
	}

	// 所属グループ一覧を取得
	groups, err := h.queries.ListUserGroups(ctx, userID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to fetch groups")
	}

	data := map[string]interface{}{
		"User":          user,
		"CurrentUserID": userIDStr,
		"UserGroups":    groups,
	}
	return c.Render(http.StatusOK, "home.html", data) // home.html を表示
}

// シフトボード画面 (特定のグループ)
func (h *Handler) ShowGroupBoard(c echo.Context) error {
	ctx := c.Request().Context()

	groupIDStr := c.Param("group_id")
	userIDStr := c.QueryParam("user_id")

	if userIDStr == "" {
		return c.String(http.StatusBadRequest, "user_id is required")
	}
	userID, _ := uuid.Parse(userIDStr)
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid group_id")
	}

	// 各種データ取得
	user, err := h.queries.GetUserByID(ctx, userID)
	if err != nil {
		return c.String(http.StatusNotFound, "User error")
	}

	group, err := h.queries.GetJobGroupByID(ctx, groupID)
	if err != nil {
		return c.String(http.StatusNotFound, "Group error")
	}

	trades, err := h.queries.ListOpenShiftTrades(ctx, groupID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Trade error")
	}

	myTrades, err := h.queries.ListUserTrades(ctx, userID)

	data := map[string]interface{}{
		"User":          user,
		"Group":         group,
		"CurrentUserID": userIDStr,
		"GroupID":       groupIDStr,
		"Trades":        trades,
		"MyTrades":      myTrades,
	}

	return c.Render(http.StatusOK, "board.html", data) // board.html を表示
}
