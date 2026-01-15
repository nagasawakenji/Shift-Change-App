-- internal/database/query.sql

-- ユーザー作成 
-- name: CreateUser :one
INSERT INTO users (line_user_id, display_name, profile_image_url)
VALUES ($1, $2, $3)
    RETURNING *;

-- LINE IDでユーザー取得
-- name: GetUserByLineID :one
SELECT * FROM users
WHERE line_user_id = $1
  AND deleted_at IS NULL
    LIMIT 1;

-- id指定でユーザー論理削除
-- name: WithdrawUser :exec
UPDATE users
SET line_user_id = $2,
    display_name = $3,
    deleted_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;

-- 退会ユーザーが作成した「募集中(OPEN)」の募集を全てCLOSEDにする
-- name: CloseOpenShiftTradesByRequester :execrows
UPDATE shift_trades
SET status = 'CLOSED',
    updated_at = NOW()
WHERE requester_id = $1
  AND status = 'OPEN';

-- グループ作成
-- name: CreateJobGroup :one
INSERT INTO job_groups (name, invitation_code, owner_id)
VALUES ($1, $2, $3)
    RETURNING *;

-- グループ参加
-- name: CreateGroupMember :one
INSERT INTO group_members (user_id, group_id, role)
VALUES ($1, $2, $3)
    RETURNING *;

-- グループ名取得
-- name: GetGroupName :one
SELECT name FROM job_groups
WHERE id = $1
  AND deleted_at IS NULL;

-- グループ所属チェック
-- name: GetGroupMember :one
SELECT gm.*
FROM group_members gm
         JOIN job_groups g ON g.id = gm.group_id
WHERE gm.group_id = $1
  AND gm.user_id = $2
  AND g.deleted_at IS NULL;

-- ユーザーが所属しているグループ一覧を取得
-- name: ListUserGroups :many
SELECT g.id, g.name, g.invitation_code, gm.role
FROM job_groups g
         JOIN group_members gm ON g.id = gm.group_id
WHERE gm.user_id = $1
  AND g.deleted_at IS NULL
ORDER BY g.created_at DESC;

-- 招待コードでグループ検索
-- name: GetJobGroupByCode :one
SELECT * FROM job_groups
WHERE invitation_code = $1
  AND deleted_at IS NULL
LIMIT 1;

-- シフト交代リクエスト作成
-- name: CreateShiftTrade :one
INSERT INTO shift_trades (
    group_id, requester_id, shift_start_at, shift_end_at, bounty_description
) VALUES (
             $1, $2, $3, $4, $5
         )
    RETURNING *;

-- そのグループの「募集中(OPEN)」のシフト一覧を取得
-- name: ListOpenShiftTrades :many
SELECT
    t.id, t.shift_start_at, t.shift_end_at, t.bounty_description, t.created_at,
    u.display_name as requester_name,
    u.profile_image_url as requester_image
FROM shift_trades t
         JOIN users u ON t.requester_id = u.id
WHERE t.group_id = $1 AND t.status = 'OPEN'
  AND t.shift_start_at > NOW()
ORDER BY t.shift_start_at ASC;

-- シフト交代リクエストの応募
-- name: AcceptShiftTrade :one
UPDATE shift_trades
SET
    acceptor_id = $1,
    status = 'FILLED',
    updated_at = NOW()
WHERE
    id = $2
    AND status = 'OPEN'
    AND requester_id != $1
    AND EXISTS (
      SELECT 1
      FROM group_members gm
      WHERE gm.user_id = $1 AND gm.group_id = $3
    )
RETURNING *;

-- シフト交代リクエストの削除
-- name: DeleteShiftTrade :execrows
DELETE FROM shift_trades
WHERE id = $1 AND requester_id = $2 AND status = 'OPEN';

-- IDでユーザー情報を取得 (画面表示用)
-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- IDでグループ情報を取得 (画面表示用)
-- name: GetJobGroupByID :one
SELECT * FROM job_groups
WHERE id = $1
  AND deleted_at IS NULL;

-- グループメンバー全員のLINE IDを取得 (通知用)
-- name: GetGroupMemberLineIDs :many
SELECT u.line_user_id
FROM group_members gm
         JOIN users u ON gm.user_id = u.id
WHERE gm.group_id = $1
  AND u.line_user_id IS NOT NULL
  AND u.line_user_id != '';

-- 自分の関わったトレード履歴を取得 (作成したもの OR 引き受けたもの)
-- name: ListUserTrades :many
SELECT * FROM shift_trades
WHERE (requester_id = $1 OR acceptor_id = $1)
ORDER BY shift_start_at DESC;

-- 謝礼を支払い済みにする
-- name: MarkTradeAsPaid :one
UPDATE shift_trades
SET is_paid = true, updated_at = NOW()
WHERE id = $1 AND requester_id = $2
    RETURNING *;

-- 指定された時間範囲にある未成立シフトを取得 (リマインド通知用)
-- name: ListUnfilledShiftsInWindow :many
SELECT t.*, u.line_user_id
FROM shift_trades t
         JOIN users u ON t.requester_id = u.id
WHERE t.status = 'OPEN'
  AND t.shift_start_at >= $1
  AND t.shift_start_at < $2;

-- シフト交代リクエストを id で取得
-- name: GetTradeByID :one
SELECT * FROM shift_trades WHERE id = $1;

-- シフト交代リクエストの詳細を編集
-- name: UpdateTradeDetails :one
UPDATE shift_trades
SET details = $3,
    updated_at = NOW()
WHERE id = $1 AND requester_id = $2
    RETURNING *;

-- グループ名を変更（ownerのみ）
-- name: UpdateJobGroupName :one
UPDATE job_groups
SET name = $2,
    updated_at = NOW()
WHERE id = $1
  AND owner_id = $3
  AND deleted_at IS NULL
RETURNING *;

-- グループを解散（論理削除）（ownerのみ）
-- name: SoftDeleteJobGroup :execrows
UPDATE job_groups
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND owner_id = $2
  AND deleted_at IS NULL;

-- 解散したグループの「募集中(OPEN)」募集を全てCLOSEDにする
-- name: CloseOpenShiftTradesByGroup :execrows
UPDATE shift_trades
SET status = 'CLOSED',
    updated_at = NOW()
WHERE group_id = $1
  AND status = 'OPEN';
