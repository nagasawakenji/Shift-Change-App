package router

import (
	"shift-change-app/internal/handler"

	"github.com/labstack/echo/v4"
)

func SetupRoutes(e *echo.Echo, h *handler.Handler) {

	api := e.Group("/api")

	// 認証が不要なAPI（必要なら最小限にする）
	api.GET("/users/:line_id", h.GetUser)

	// 認証が必要なAPI（Authorization: Bearer <token>）
	authed := api.Group("")
	authed.Use(handler.AuthMiddleware())
	{
		authed.POST("/users", h.RegisterUser)
		authed.POST("/groups", h.CreateGroup)
		authed.POST("/groups/join", h.JoinGroup)
		authed.POST("/me", h.Me)

		authed.POST("/groups/:group_id/trades", h.CreateTrade)
		authed.GET("/groups/:group_id/trades", h.ListTrades)
		authed.DELETE("/groups/:group_id/trades/:trade_id", h.DeleteTrade)
		authed.PUT("/groups/:group_id/trades/:trade_id/accept", h.AcceptTrade)
		authed.PUT("/trades/:trade_id/paid", h.MarkPaid)
		authed.PUT("/groups/:group_id/trades/:trade_id/details", h.UpdateTradeDetails)
	}

	// 画面表示 (HTML)
	e.GET("/", func(c echo.Context) error {
		st := c.QueryParam("liff.state")
		// LIFF は/registerをクエリパラメーターとして渡す
		if st == "/register" {
			return h.ShowRegister(c)
		}

		return h.ShowHomeEntry(c)
	})

	e.GET("/groups/:group_id", h.ShowGroupBoard)
	e.POST("/callback", h.Webhook)

	// 登録関連
	e.GET("/register", h.ShowRegister)

	// ホーム画面
	e.GET("/home", func(c echo.Context) error {
		// まだ user_id が付いてないなら入口へ
		if c.QueryParam("user_id") == "" {
			return h.ShowHomeEntry(c)
		}
		return h.ShowHome(c)
	})

	e.GET("/groups/:group_id/trades/:trade_id", h.ShowTradeDetail)
}
