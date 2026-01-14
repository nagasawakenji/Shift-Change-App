-- migrations/000001_create_table.up.sql

-- UUID生成用の拡張機能を有効化
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ユーザー
CREATE TABLE users (
                       id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                       line_user_id VARCHAR(255) NOT NULL UNIQUE,
                       display_name VARCHAR(255) NOT NULL,
                       profile_image_url TEXT,
                       created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                       updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_line_id ON users(line_user_id);

-- バイトグループ
CREATE TABLE job_groups (
                            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                            name VARCHAR(255) NOT NULL,
                            invitation_code VARCHAR(50) NOT NULL UNIQUE,
                            owner_id UUID NOT NULL REFERENCES users(id),
                            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- グループメンバー（中間テーブル）
CREATE TABLE group_members (
                               user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                               group_id UUID NOT NULL REFERENCES job_groups(id) ON DELETE CASCADE,
                               role VARCHAR(20) NOT NULL DEFAULT 'MEMBER',
                               joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                               PRIMARY KEY (user_id, group_id)
);

-- シフト交代リクエスト
CREATE TABLE shift_trades (
                              id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                              group_id UUID NOT NULL REFERENCES job_groups(id) ON DELETE CASCADE,
                              requester_id UUID NOT NULL REFERENCES users(id),
                              acceptor_id UUID REFERENCES users(id),
                              shift_start_at TIMESTAMPTZ NOT NULL,
                              shift_end_at TIMESTAMPTZ NOT NULL,
                              bounty_description TEXT NOT NULL DEFAULT '',
                              status VARCHAR(20) NOT NULL DEFAULT 'OPEN',
                              created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                              updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_trades_group_status ON shift_trades(group_id, status);