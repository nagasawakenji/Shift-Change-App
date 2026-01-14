-- migrations/000001_create_table.down.sql

DROP TABLE IF EXISTS shift_trades;
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS job_groups;
DROP TABLE IF EXISTS users;
DROP EXTENSION IF EXISTS "uuid-ossp";