// cmd/api/cron.go
package main

import (
	"context"
	"log"
	"time"

	"shift-change-app/internal/database"

	"github.com/line/line-bot-sdk-go/v7/linebot"
)

// 10分ごとに未成立シフトをチェックする
func StartReminderWorker(queries *database.Queries, bot *linebot.Client) {
	// 10分間隔のタイマーを作成
	ticker := time.NewTicker(10 * time.Minute)

	go func() {
		for {
			select {
			case <-ticker.C:
				checkAndNotify(queries, bot)
			}
		}
	}()
}

func checkAndNotify(queries *database.Queries, bot *linebot.Client) {
	ctx := context.Background()
	now := time.Now()

	// 5時間後 ~ 5時間10分後 の範囲を計算
	targetStart := now.Add(5 * time.Hour)
	targetEnd := targetStart.Add(10 * time.Minute)

	shifts, err := queries.ListUnfilledShiftsInWindow(ctx, database.ListUnfilledShiftsInWindowParams{
		ShiftStartAt:   targetStart,
		ShiftStartAt_2: targetEnd,
	})
	if err != nil {
		log.Println("Error checking shifts:", err)
		return
	}

	// 対象があれば通知
	for _, shift := range shifts {
		if shift.LineUserID != "" {
			msg := "⚠️ 【重要】シフト成立期限が迫っています\n\n" +
				"日時: " + shift.ShiftStartAt.Format("15:04") + " ~\n\n" +
				"開始5時間前になりましたが、まだ代わりの人が見つかっていません。\n" +
				"至急、バイト先に連絡しましょう！"

			if _, err := bot.PushMessage(shift.LineUserID, linebot.NewTextMessage(msg)).Do(); err != nil {
				log.Println("Failed to send reminder:", err)
			} else {
				log.Printf("Sent reminder to user %s for trade %s", shift.LineUserID, shift.ID)
			}
		}
	}
}
