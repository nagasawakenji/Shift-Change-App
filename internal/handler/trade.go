package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"shift-change-app/internal/database"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/line/line-bot-sdk-go/v7/linebot"
)

// JST 表示用（DBはUTC保存のままでOK）
var jst = time.FixedZone("Asia/Tokyo", 9*60*60)

func formatShiftRangeJST(start, end time.Time) string {
	s := start.In(jst)
	e := end.In(jst)
	// 同日なら終了側は時刻だけにして読みやすく
	if s.Format("01/02") == e.Format("01/02") {
		return s.Format("01/02 15:04") + " ~ " + e.Format("15:04")
	}
	return s.Format("01/02 15:04") + " ~ " + e.Format("01/02 15:04")
}

func formatDateJST(t time.Time) string {
	return t.In(jst).Format("01/02")
}

// LINE userId (sub) の簡易バリデーション
// 典型的には `U` + 32桁のhex 文字列。
func isValidLineUserID(id string) bool {
	id = strings.TrimSpace(id)
	if len(id) != 33 || !strings.HasPrefix(id, "U") {
		return false
	}
	for _, ch := range id[1:] {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return true
}

func filterValidLineUserIDs(ids []string) (valid []string, skipped int) {
	valid = make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			skipped++
			continue
		}
		if !isValidLineUserID(id) {
			skipped++
			continue
		}
		valid = append(valid, id)
	}
	return valid, skipped
}

// devバイパス経由のリクエストかどうか
// AuthMiddleware が dev バイパスで認証した場合は context に `dev_bypass=true` をセットする。
func isDevBypassRequest(c echo.Context) bool {
	if v := c.Get("dev_bypass"); v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	// フォールバック: 直接ヘッダを見る（古い経路・直叩き用）
	return strings.TrimSpace(c.Request().Header.Get("X-Dev-Sub")) != ""
}

// middleware がセットした sub(=LINE userId) からアプリ内 userUUID を確定する
func (h *Handler) userUUIDFromAuth(c echo.Context) (uuid.UUID, error) {
	ctx := c.Request().Context()

	sub, ok := LineSub(c)
	if !ok {
		return uuid.Nil, errors.New("unauthorized")
	}

	user, err := h.queries.GetUserByLineID(ctx, sub)
	if err != nil {
		return uuid.Nil, err
	}
	return user.ID, nil
}

// シフト交代リクエスト作成
func (h *Handler) CreateTrade(c echo.Context) error {
	ctx := c.Request().Context()

	// バイトグループid を取得する
	groupIDStr := c.Param("group_id")
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request format"})
	}

	groupName, err := h.queries.GetGroupName(ctx, groupID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request format"})
	}

	type Request struct {
		StartAt time.Time `json:"start_at"`
		EndAt   time.Time `json:"end_at"`
		Bounty  string    `json:"bounty"`
	}
	var req Request
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request format"})
	}

	userUUID, err := h.userUUIDFromAuth(c)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not registered"})
		}
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	// リクエストユーザーがグループに所属しているかを判定する
	_, err = h.queries.GetGroupMember(ctx, database.GetGroupMemberParams{
		GroupID: groupID,
		UserID:  userUUID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// シフト交換リクエストの作成
	trade, err := h.queries.CreateShiftTrade(ctx, database.CreateShiftTradeParams{
		GroupID:           groupID,
		RequesterID:       userUUID,
		ShiftStartAt:      req.StartAt,
		ShiftEndAt:        req.EndAt,
		BountyDescription: req.Bounty,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create trade: " + err.Error()})
	}

	// bot でシフト交換リクエストの作成を通知する（devバイパス時は送らない）
	if isDevBypassRequest(c) {
		c.Logger().Info("[notify] skip multicast in dev-bypass request")
		return c.JSON(http.StatusOK, trade)
	}
	go func() {

		ctx := context.Background()

		lineIDs, err := h.queries.GetGroupMemberLineIDs(ctx, groupID)
		if err != nil {
			c.Logger().Error("Failed to get member line IDs:", err)
			return
		}

		var to []string
		for _, id := range lineIDs {
			to = append(to, id)
		}
		to, skipped := filterValidLineUserIDs(to)
		if skipped > 0 {
			c.Logger().Warnf("[notify] multicast: skipped %d invalid line_user_id(s)", skipped)
		}

		if len(to) > 0 {
			// bot から送信されるメッセージ
			msg := "📢 新しいシフト募集があります！\n\n" +
				"グループ: " + groupName + "\n\n" +
				"日時: " + formatShiftRangeJST(req.StartAt, req.EndAt) + "\n" +
				"謝礼: " + req.Bounty + "\n\n" +
				"アプリから確認してください！"

			// Multicast で一斉送信
			if _, err := h.bot.Multicast(to, linebot.NewTextMessage(msg)).Do(); err != nil {
				c.Logger().Error("Failed to send multicast:", err)
			}
		}

	}()

	return c.JSON(http.StatusOK, trade)

}

// 募集中のシフト交代リクエストを一覧取得
func (h *Handler) ListTrades(c echo.Context) error {
	ctx := c.Request().Context()

	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid group_id"})
	}

	userUUID, err := h.userUUIDFromAuth(c)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not registered"})
		}
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	// 所属チェック
	if _, err := h.queries.GetGroupMember(ctx, database.GetGroupMemberParams{
		GroupID: groupID,
		UserID:  userUUID,
	}); err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You are not a member of this group"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	trades, err := h.queries.ListOpenShiftTrades(ctx, groupID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, trades)
}

// 状態が OPEN のシフト交代リクエストの削除
func (h *Handler) DeleteTrade(c echo.Context) error {
	ctx := c.Request().Context()

	tradeID, err := uuid.Parse(c.Param("trade_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid trade_id"})
	}

	userUUID, err := h.userUUIDFromAuth(c)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not registered"})
		}
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	count, err := h.queries.DeleteShiftTrade(ctx, database.DeleteShiftTradeParams{
		ID:          tradeID,
		RequesterID: userUUID,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if count == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Cannot delete trade. Either it does not exist, it's not yours, or it's already filled."})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "Trade deleted successfully"})
}

// シフト交代リクエストの応募
func (h *Handler) AcceptTrade(c echo.Context) error {
	ctx := c.Request().Context()

	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid group_id"})
	}
	tradeID, err := uuid.Parse(c.Param("trade_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid trade_id"})
	}

	acceptorUUID, err := h.userUUIDFromAuth(c)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not registered"})
		}
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	// 所属チェック（AcceptShiftTradeのSQL内でチェックしてるなら省略しても良いが、入れると明快）
	if _, err := h.queries.GetGroupMember(ctx, database.GetGroupMemberParams{
		GroupID: groupID,
		UserID:  acceptorUUID,
	}); err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You are not a member of this group"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	trade, err := h.queries.AcceptShiftTrade(ctx, database.AcceptShiftTradeParams{
		AcceptorID: uuid.NullUUID{UUID: acceptorUUID, Valid: true},
		ID:         tradeID,
		GroupID:    groupID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "Cannot accept trade. Possible reasons: trade not found, already filled, it's your own request, or you are not a member.",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// 通知（devバイパス時は送らない）
	if isDevBypassRequest(c) {
		c.Logger().Info("[notify] skip push in dev-bypass request")
		return c.JSON(http.StatusOK, trade)
	}
	go func() {
		ctx := context.Background()

		var acceptorName string
		var acceptorLineID string

		if trade.AcceptorID.Valid {
			acceptor, err := h.queries.GetUserByID(ctx, trade.AcceptorID.UUID)
			if err == nil {
				acceptorName = acceptor.DisplayName
				if isValidLineUserID(acceptor.LineUserID) {
					acceptorLineID = acceptor.LineUserID
				}
			} else {
				acceptorName = "メンバー"
			}
		}

		requester, err := h.queries.GetUserByID(ctx, trade.RequesterID)
		if err == nil && isValidLineUserID(requester.LineUserID) {
			// メッセージに相手の名前を入れる
			msg := "🎉 シフトが成立しました！\n\n" +
				"日時: " + formatShiftRangeJST(trade.ShiftStartAt, trade.ShiftEndAt) + "\n" +
				"相手: " + acceptorName + " さん\n\n" +
				"あなたのシフト募集が引き受けられました。\n" +
				"引き継ぎや業務内容など、詳細を追記するとスムーズです。\n" +
				"（詳細ページから追記できます）"

			if _, err := h.bot.PushMessage(requester.LineUserID, linebot.NewTextMessage(msg)).Do(); err != nil {
				c.Logger().Error("Failed to push to requester:", err)
			}
		}

		if isValidLineUserID(acceptorLineID) {
			msg := "👍 シフトを引き受けました！\n\n" +
				"日時: " + formatShiftRangeJST(trade.ShiftStartAt, trade.ShiftEndAt) + "\n" +
				"当日よろしくおねがいします！"

			if _, err := h.bot.PushMessage(acceptorLineID, linebot.NewTextMessage(msg)).Do(); err != nil {
				c.Logger().Error("Failed to push to acceptor:", err)
			}
		}
	}()

	return c.JSON(http.StatusOK, trade)
}

// 謝礼支払い完了
func (h *Handler) MarkPaid(c echo.Context) error {
	ctx := c.Request().Context()

	tradeID, err := uuid.Parse(c.Param("trade_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid trade_id"})
	}

	userUUID, err := h.userUUIDFromAuth(c)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not registered"})
		}
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	trade, err := h.queries.MarkTradeAsPaid(ctx, database.MarkTradeAsPaidParams{
		ID:          tradeID,
		RequesterID: userUUID,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update payment status"})
	}

	// 支払い通知の送信（devバイパス時は送らない）
	if isDevBypassRequest(c) {
		c.Logger().Info("[notify] skip paid notification in dev-bypass request")
		return c.JSON(http.StatusOK, trade)
	}
	go func() {
		if trade.AcceptorID.Valid {
			acceptor, err := h.queries.GetUserByID(context.Background(), trade.AcceptorID.UUID)
			if err == nil && isValidLineUserID(acceptor.LineUserID) {
				requester, _ := h.queries.GetUserByID(context.Background(), trade.RequesterID)
				requesterName := requester.DisplayName

				msg := "💰 謝礼の支払いが記録されました！\n\n" +
					"支払者: " + requesterName + "\n" +
					"日時: " + formatDateJST(trade.ShiftStartAt) + " のシフト\n\n" +
					"手渡し、または送金アプリ等で着金を確認してください。"

				h.bot.PushMessage(acceptor.LineUserID, linebot.NewTextMessage(msg)).Do()
			}
		}
	}()

	return c.JSON(http.StatusOK, trade)

}
