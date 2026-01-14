package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

type lineVerifyResp struct {
	Sub  string `json:"sub"` // LINE userId
	Aud  string `json:"aud"`
	Iss  string `json:"iss"`
	Exp  int64  `json:"exp"`
	Iat  int64  `json:"iat"`
	Name string `json:"name,omitempty"`
}

// user_id の取得（Authorizationで認証済み）
func (h *Handler) Me(c echo.Context) error {
	ctx := c.Request().Context()

	sub, ok := LineSub(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	user, err := h.queries.GetUserByLineID(ctx, sub)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not registered"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch user"})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"user_id": user.ID.String(),
	})
}

// idToken を LINE の verify API で検証し、LINE userId（sub）を返す
func verifyLineIDToken(idToken string) (string, error) {
	if idToken == "" {
		return "", fmt.Errorf("id_token is required")
	}
	clientID := os.Getenv("LINE_LOGIN_CHANNEL_ID")
	if clientID == "" {
		return "", fmt.Errorf("LINE_LOGIN_CHANNEL_ID is not set")
	}

	form := url.Values{}
	form.Set("id_token", idToken)
	form.Set("client_id", clientID)

	res, err := http.Post(
		"https://api.line.me/oauth2/v2.1/verify",
		"application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("failed to call verify api: %w", err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		// body に LINE からのエラーJSONが入ることが多い
		return "", fmt.Errorf("verify failed: status=%d body=%s", res.StatusCode, string(body))
	}

	var v lineVerifyResp
	if err := json.Unmarshal(body, &v); err != nil {
		return "", fmt.Errorf("failed to parse verify response: %w", err)
	}
	if v.Sub == "" {
		return "", fmt.Errorf("verify response missing sub")
	}
	return v.Sub, nil
}
