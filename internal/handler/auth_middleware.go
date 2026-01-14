package handler

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

type ctxKey string

const (
	ctxLineSub   ctxKey = "line_sub"
	ctxDevBypass ctxKey = "dev_bypass"
)

// 認証済みの場合、line_id を返却する
func LineSub(c echo.Context) (string, bool) {
	v := c.Get(string(ctxLineSub))
	s, ok := v.(string)
	return s, ok
}

// 認証
// ヘッダーに付属した Authorization: Bearer <id_token> を LINE verify API で検証する
// 開発環境では Authorization: Bearer <DEV_AUTH_TOKEN> となっていた時に限り X-Dev-Sub を Sub として検証を通過する
func AuthMiddleware() echo.MiddlewareFunc {
	appEnv := strings.ToLower(os.Getenv("APP_ENV"))
	devAuthToken := strings.TrimSpace(os.Getenv("DEV_AUTH_TOKEN"))

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authz := c.Request().Header.Get("Authorization")
			// Bearer でない場合は弾く
			if !strings.HasPrefix(authz, "Bearer ") {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing Authorization: Bearer token"})
			}

			// トリム
			bearer := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))

			if strings.TrimSpace(os.Getenv("AUTH_DEBUG")) == "1" {
				c.Logger().Infof("[AUTH_DEBUG] APP_ENV=%q devTokenSet=%v authzPrefix=%q bearerLen=%d devLen=%d",
					appEnv,
					devAuthToken != "",
					func() string {
						if len(authz) > 12 {
							return authz[:12]
						}
						return authz
					}(),
					len(bearer),
					len(devAuthToken),
				)
			}

			// dev 環境で Authorization: Bearer <DEV_AUTH_TOKEN> を使用する場合
			if appEnv != "prod" && devAuthToken != "" {
				okCmp := subtle.ConstantTimeCompare([]byte(bearer), []byte(devAuthToken)) == 1
				if strings.TrimSpace(os.Getenv("AUTH_DEBUG")) == "1" {
					c.Logger().Infof("[AUTH_DEBUG] dev compare ok=%v", okCmp)
				}
				if okCmp {
					sub := strings.TrimSpace(c.Request().Header.Get("X-Dev-Sub"))
					if sub == "" {
						return c.JSON(http.StatusBadRequest, map[string]string{"error": "X-Dev-Sub is required for dev auth"})
					}
					// middleware と同じキーにセット
					c.Set(string(ctxLineSub), sub)
					c.Set(string(ctxDevBypass), true)
					if strings.TrimSpace(os.Getenv("AUTH_DEBUG")) == "1" {
						c.Logger().Infof("[AUTH_DEBUG] dev bypass success (subLen=%d)", len(sub))
					}
					return next(c)
				}
			}

			// LINE verify API を用いた検証
			sub, err := verifyLineIDToken(bearer)
			if strings.TrimSpace(os.Getenv("AUTH_DEBUG")) == "1" && err == nil {
				c.Logger().Infof("[AUTH_DEBUG] line verify success (subLen=%d)", len(sub))
			}
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid id_token"})
			}
			c.Set(string(ctxLineSub), sub)
			c.Set(string(ctxDevBypass), false)
			return next(c)
		}
	}
}
