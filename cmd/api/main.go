package main

import (
	"database/sql"
	"html/template"
	"io"
	"log"
	"os"
	"shift-change-app/internal/database"
	"shift-change-app/internal/handler"
	"shift-change-app/internal/router"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"

	"github.com/line/line-bot-sdk-go/v7/linebot"
)

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// データベース接続
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("failed to open db connection:", err)
	}
	defer db.Close()

	queries := database.New(db)

	channelToken := os.Getenv("CHANNEL_TOKEN")
	if channelToken == "" {
		log.Fatal("CHANNEL_TOKEN is not set")
	}
	channelSecret := os.Getenv("CHANNEL_SECRET")
	if channelSecret == "" {
		log.Fatal("CHANNEL_SECRET is not set")
	}

	bot, err := linebot.New(channelSecret, channelToken)
	if err != nil {
		log.Fatal(err)
	}

	h := handler.NewHandler(db, queries, bot)

	StartReminderWorker(queries, bot)

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	t := &Template{
		// viewsフォルダにある .html ファイルを全て読み込む
		templates: template.Must(template.ParseGlob("views/*.html")),
	}
	e.Renderer = t

	router.SetupRoutes(e, h)

	log.Printf("[BOOT] APP_ENV=%q DEV_AUTH_TOKEN=%q", os.Getenv("APP_ENV"), os.Getenv("DEV_AUTH_TOKEN"))
	e.Logger.Infof("APP_ENV=%q DEV_AUTH_TOKEN=%q", os.Getenv("APP_ENV"), os.Getenv("DEV_AUTH_TOKEN"))

	// Render は PORT 環境変数で待ち受けポートを渡してくる
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // ローカル用フォールバック
	}
	log.Println("[BOOT] about to start server on PORT =", port)

	e.Logger.Fatal(e.Start(":" + port))
}
