package handler

import (
	"database/sql"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/line/line-bot-sdk-go/v7/linebot"
)

func (h *Handler) Webhook(c echo.Context) error {
	req := c.Request()

	events, err := h.bot.ParseRequest(req)
	if err != nil {
		// 署名不正など。
		c.Logger().Errorf("ParseRequest error: %v", err)
		if err == linebot.ErrInvalidSignature {
			return c.NoContent(http.StatusBadRequest)
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	registerURL := os.Getenv("REGISTER_URL")
	if registerURL == "" {
		c.Logger().Warn("REGISTER_URL is not set")
	}

	for _, event := range events {
		userID := ""
		if event.Source != nil {
			userID = event.Source.UserID
		}
		if userID == "" {
			continue
		}

		// 既に登録済みかを判定（
		_, user := h.queries.GetUserByLineID(req.Context(), userID)
		registered := true
		if user != nil {
			if user == sql.ErrNoRows {
				registered = false
			} else {
				// DBエラー等。とりあえずログだけ出して次へ
				c.Logger().Errorf("GetUserByLineID error: %v", user)
				continue
			}
		}

		// 未登録のときだけ案内を返す
		if !registered {
			switch event.Type {
			case linebot.EventTypeFollow:
				// 友だち追加直後
				msg := linebot.NewTextMessage(
					"友だち追加ありがとうございます！🙇\n\n" +
						"シフト管理アプリへようこそ。\n" +
						"まずは以下から利用登録を完了させてください！\n" +
						registerURL,
				)
				if _, err := h.bot.ReplyMessage(event.ReplyToken, msg).Do(); err != nil {
					c.Logger().Error(err)
				}

			case linebot.EventTypeMessage:
				// ブロック解除後など、ユーザーが何か送ってきたタイミングで案内
				msg := linebot.NewTextMessage(
					"利用には登録が必要です！\nこちらから登録してください👇\n" + registerURL,
				)
				if _, err := h.bot.ReplyMessage(event.ReplyToken, msg).Do(); err != nil {
					c.Logger().Error(err)
				}
			}
		}

	}

	return c.NoContent(http.StatusOK)
}
