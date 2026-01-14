package handler

import (
	"database/sql"
	"github.com/line/line-bot-sdk-go/v7/linebot"
	"shift-change-app/internal/database"
)

type Handler struct {
	db      *sql.DB
	queries *database.Queries
	bot     *linebot.Client
}

func NewHandler(db *sql.DB, queries *database.Queries, bot *linebot.Client) *Handler {
	return &Handler{
		db:      db,
		queries: queries,
		bot:     bot,
	}
}
