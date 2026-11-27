"""Generate the approved 543-player fantasy pool, retaining source provenance."""
import csv
from collections import Counter
from pathlib import Path

DATA = Path(__file__).resolve().parent / 'datasets'
ROLE = {'BATTER':'BATTER','BOWLER':'BOWLER','ALL_ROUNDER':'ALL_ROUNDER','WICKET_KEEPER':'WICKETKEEPER'}

def category(row):
    role = row['specialism'].upper().replace('-', '_').replace('WICKETKEEPER','WICKET_KEEPER')
    status = row['effective_game_capped_status'].upper()
    if status == 'ASSOCIATE' and role == 'ALL_ROUNDER':
        return 'ASSOCIATE_ALL_ROUNDER'
    if status not in ('CAPPED','UNCAPPED') or role not in ROLE:
        raise ValueError(f"Invalid classification: {row['player_name_official']}")
    return status + '_' + ROLE[role]

def generate():
    with (DATA/'players_master_roles_updated.csv').open(newline='', encoding='utf-8-sig') as f:
        rows = list(csv.DictReader(f))
    names = [r['player_name_official'].strip().casefold() for r in rows]
    if len(rows) != 543 or len(set(names)) != 543 or not all(names):
        raise ValueError('Expected 543 unique player identities')
    output = [{'full_name':r['player_name_official'].strip(), 'auction_category':category(r), 'is_auction_eligible':'true'} for r in rows]
    with (DATA/'player_auction_metadata_543.csv').open('w', newline='', encoding='utf-8') as f:
        w = csv.DictWriter(f, fieldnames=list(output[0])); w.writeheader(); w.writerows(output)
    print('Total auction eligible:',len(output))
    print('Categories:',dict(Counter(r['auction_category'] for r in output)))

if __name__ == '__main__':
    generate()
