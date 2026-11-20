
-- Global player auction metadata
ALTER TABLE players
ADD COLUMN auction_category VARCHAR(50),
ADD COLUMN is_auction_eligible BOOLEAN NOT NULL DEFAULT FALSE;

-- Room-specific auction order
ALTER TABLE room_players
ADD COLUMN auction_sequence INTEGER;

ALTER TABLE players
ADD CONSTRAINT players_auction_category_check
CHECK (
    auction_category IS NULL
    OR auction_category IN (
        'MARQUEE',
        'CAPPED_BATTER',
        'CAPPED_BOWLER',
        'CAPPED_ALL_ROUNDER',
        'CAPPED_WICKETKEEPER',
        'UNCAPPED_BATTER',
        'UNCAPPED_BOWLER',
        'UNCAPPED_ALL_ROUNDER',
        'UNCAPPED_WICKETKEEPER',
        'ASSOCIATE_ALL_ROUNDER'
    )
);

ALTER TABLE room_players
ADD CONSTRAINT room_players_auction_sequence_check
CHECK (
    auction_sequence IS NULL OR auction_sequence > 0
);

CREATE INDEX idx_players_auction_eligible_category
ON players (auction_category)
WHERE is_auction_eligible = TRUE;

CREATE INDEX idx_room_players_auction_queue
ON room_players (room_id, auction_sequence)
WHERE auction_sequence IS NOT NULL;
