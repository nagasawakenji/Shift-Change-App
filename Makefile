# DBコンテナをバックグラウンドで起動
db-up:
	docker-compose up -d db

# DBコンテナを停止・削除
db-down:
	docker-compose down

# マイグレーション実行（テーブル作成）
# ローカルにインストールした migrate ツールを使用
migrate-up:
	migrate -path migrations -database "postgres://user:password@localhost:5432/shift_app?sslmode=disable" up

# マイグレーション取り消し（テーブル削除）
migrate-down:
	migrate -path migrations -database "postgres://user:password@localhost:5432/shift_app?sslmode=disable" down

# DBの状態確認
db-status:
	docker ps --filter "name=shift_app_db"

# サーバー起動 (Airによるホットリロード)
dev:
	air

# sqlcによるGoコード生成
sqlc:
	sqlc generate

# テスト実行
test:
	go test -v ./...