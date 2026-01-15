# Shift Change App

LINE(LIFF)上で動作する、アルバイト向けの「シフト代替」調整アプリです。  
シフト代替の作成、引き受け、通知、未成立リマインドまでを一通り揃えています。  

___
## 主な機能

- ユーザー登録（LINE ID Token を検証）
- グループ作成 / 招待コードによる参加
- シフト募集の作成・一覧表示
- シフト募集の詳細表示（作成者のみ詳細編集）
- シフト引き受け（成立）
- 支払い完了マーク（作成者のみ）
- 未成立シフトのリマインド通知（シフト開始5時間前）
- 退会（匿名化 + 退会者の募集を無効化）

___

## アーキテクチャ図
![アーキテクチャ図](https://github.com/user-attachments/assets/beaf0005-39bd-418c-9908-7497743ce440)

___
## 技術スタック

•	Backend: Go + Echo  
•	Frontend: HTML + Tailwind CSS + JavaScript（LIFF）  
•	DB: PostgreSQL（Supabase）  
•	Bot / Notification: LINE Messaging API  
•	Auth: LINE Login（ID Token verify API）  
•	Deploy: Render  

___
## ディレクトリ構成 (概要)
```aiignore
cmd/api            # APIサーバー起動
internal/handler   # API/HTMLのハンドラ
internal/router    # ルーティング
internal/database  # sqlcで生成したDBアクセス
views              # HTML（LIFF画面）
migrations         # DBマイグレーション
```

___
## 環境変数

### 共通

| 変数名 | 説明 |
|------|------|
| DATABASE_URL | PostgreSQL接続URL |
| CHANNEL_SECRET | LINE Messaging API の Channel Secret |
| CHANNEL_TOKEN | LINE Messaging API の Channel Access Token |
| LINE_LOGIN_CHANNEL_ID | LINE Login の Channel ID（ID Token Verifyに使用） |
| APP_ENV | dev / prod |
| PORT | Render が注入する待受ポート（ローカルは無くても動きます） |

### 開発用

| 変数名 | 説明 |
|------|------|
| DEV_AUTH_TOKEN | dev用固定トークン（Middlewareがこれを許可する） |

___
## ローカル起動
### 1. DB起動 (docker-compose)
```bash
docker compose up -d
```
### 2. 環境変数設定(.envに設定しても良いです)
```bash
export APP_ENV=dev
export DEV_AUTH_TOKEN=dev_auth_token_for_developer
export DATABASE_URL="postgresql://user:pass@localhost:5432/shift_app?sslmode=disable"

export LINE_LOGIN_CHANNEL_ID="..."
export CHANNEL_SECRET="..."
export CHANNEL_TOKEN="..."
```
### 3. 起動
```bash
make dev
```

___

## 認証方式

本アプリは Echo Middleware によって認証を統一しています。

### 本番（通常の認証）  
•	クライアント（LIFF）が id_token を取得   
•	API呼び出し時に Authorization: Bearer <id_token> を付与   
•	サーバーが LINE Verify API に投げて検証し、sub（LINE user id）を取得   

### dev 環境での curl 認証(重要)
開発中は LINE アカウントを複数用意しづらいので、
dev環境では ID Token の検証をバイパスできる仕組みを用意しています。   

条件   
•	APP_ENV != prod   
•	Authorization: Bearer <DEV_AUTH_TOKEN>   
•	X-Dev-Sub: <任意のLINE user id> を付与   

この場合、LINE Verify API を呼ばずに X-Dev-Sub をそのまま sub として採用します。
___

### curl 例 (ユーザー登録)
```bash
curl -s -X POST http://localhost:8080/api/users \
  -H "Authorization: Bearer dev_auth_token_for_developer" \
  -H "X-Dev-Sub: U111" \
  -H "Content-Type: application/json" \
  -d '{"name":"DevTaro"}' | jq
```
___
### curl 例 (グループ作成)
```bash
curl -s -X POST http://localhost:8080/api/groups \
  -H "Authorization: Bearer dev_auth_token_for_developer" \
  -H "X-Dev-Sub: U111" \
  -H "Content-Type: application/json" \
  -d '{"group_name":"新店舗ABC"}' | jq
```
___
### curl 例 (シフト募集の作成)
```bash
curl -s -X POST http://localhost:8080/api/groups/<GROUP_ID>/trades \
  -H "Authorization: Bearer dev_auth_token_for_developer" \
  -H "X-Dev-Sub: U111" \
  -H "Content-Type: application/json" \
  -d '{"start_at":"2026-01-15T10:00:00+09:00","end_at":"2026-01-15T14:00:00+09:00","bounty":"500円"}' | jq
```
___
### curl 例 (募集の引き受け)
```bash
curl -s -X PUT http://localhost:8080/api/groups/<GROUP_ID>/trades/<TRADE_ID>/accept \
  -H "Authorization: Bearer dev_auth_token_for_developer" \
  -H "X-Dev-Sub: U111" \
  -H "Content-Type: application/json" | jq
```

___
## API (概要)
主要な API を抜粋しています

### 主要 API 一覧

| Method | Path | 説明 |
|------|------|------|
| POST | /api/users | ユーザー登録 |
| GET | /api/users/:line_id | ユーザー取得（公開） |
| POST | /api/groups | グループ作成 |
| POST | /api/groups/join | 招待コードで参加 |
| POST | /api/me | 自分の user_id 取得 |
| DELETE | /api/me | 退会 |
| POST | /api/groups/:group_id/trades | 募集作成 |
| GET | /api/groups/:group_id/trades | 一覧取得 |
| PUT | /api/groups/:group_id/trades/:trade_id/accept | 引き受け |
| DELETE | /api/groups/:group_id/trades/:trade_id | 募集削除 |
| PUT | /api/trades/:trade_id/paid | 支払い完了 |
| PUT | /api/groups/:group_id/trades/:trade_id/details | 詳細更新 |


___
## 注意事項
•	LINE Verify API の検証は LINE_LOGIN_CHANNEL_ID が一致しないと失敗します  
（Renderで 401 になる場合は、まずここを確認してください）  
•	devバイパス（DEV_AUTH_TOKEN + X-Dev-Sub）は prodでは無効化されます

⸻



