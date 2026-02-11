-- Migration 001: Remove denormalized archive columns from shows table
-- These columns (archive_current_date, archive_current_m3u_url,
-- archive_previous_date, archive_previous_m3u_url) stored cached archive
-- data that belongs in the archive scraping layer, not in the shows table.
--
-- Requires SQLite 3.35.0+ (DROP COLUMN support)
-- Current data in these columns will be lost.

ALTER TABLE shows DROP COLUMN archive_current_date;
ALTER TABLE shows DROP COLUMN archive_current_m3u_url;
ALTER TABLE shows DROP COLUMN archive_previous_date;
ALTER TABLE shows DROP COLUMN archive_previous_m3u_url;
