
DROP INDEX IF EXISTS idx_room_players_auction_queue;
DROP INDEX IF EXISTS idx_players_auction_eligible_category;

ALTER TABLE room_players
DROP CONSTRAINT IF EXISTS room_players_auction_sequence_check;

ALTER TABLE room_players
DROP COLUMN IF EXISTS auction_sequence;

ALTER TABLE players
DROP CONSTRAINT IF EXISTS players_auction_category_check;

ALTER TABLE players
DROP COLUMN IF EXISTS is_auction_eligible,
DROP COLUMN IF EXISTS auction_category;
