ALTER TABLE players
DROP CONSTRAINT players_auction_category_check;

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