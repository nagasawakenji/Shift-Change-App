package handler

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

// 登録画面表示
func (h *Handler) ShowRegister(c echo.Context) error {
	groupID := c.QueryParam("group_id")
	return c.Render(http.StatusOK, "register.html", map[string]interface{}{
		"GroupID": groupID,
		"LiffID":  os.Getenv("LIFF_ID"),
	})
}
