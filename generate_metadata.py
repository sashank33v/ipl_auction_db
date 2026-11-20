
import csv
from collections import Counter

SOURCE = "datasets/players_master_roles_updated.csv"
OUTPUT = "datasets/player_auction_metadata_543.csv"

category_map = {
    "Capped Batter": "CAPPED_BATTER",
    "Capped Bowler": "CAPPED_BOWLER",
    "Capped All-rounder": "CAPPED_ALL_ROUNDER",
    "Capped Wicketkeeper": "CAPPED_WICKETKEEPER",
    "Uncapped Batter": "UNCAPPED_BATTER",
    "Uncapped Bowler": "UNCAPPED_BOWLER",
    "Uncapped All-rounder": "UNCAPPED_ALL_ROUNDER",
    "Uncapped Wicketkeeper": "UNCAPPED_WICKETKEEPER",
    "Associate All-rounder": "ASSOCIATE_ALL_ROUNDER",
}

reference_category_map = {
    "Reference Only - Batter": "REFERENCE_BATTER",
    "Reference Only - Bowler": "REFERENCE_BOWLER",
    "Reference Only - All-rounder": "REFERENCE_ALL_ROUNDER",
    "Reference Only - Wicketkeeper": "REFERENCE_WICKETKEEPER",
}

with open(SOURCE, newline="", encoding="utf-8-sig") as f:
    rows = list(csv.DictReader(f))

if len(rows) != 543:
    raise ValueError(f"Expected 543 rows, found {len(rows)}")

output_rows = []
names = set()
category_counts = Counter()
eligible_count = 0
reference_count = 0

for row in rows:
    name = row["player_name_official"].strip()
    record_type = row["dataset_record_type"].strip()
    source_eligible = (
        row["auction_eligible_in_this_dataset"].strip().lower() == "true"
    )

    if not name:
        raise ValueError("Empty player name found")

    normalized_name = name.casefold()
    if normalized_name in names:
        raise ValueError(f"Duplicate player name: {name}")
    names.add(normalized_name)

    if record_type == "AUCTION_CANDIDATE":
        if not source_eligible:
            raise ValueError(f"Unexpected ineligible candidate: {name}")

        source_category = row["auction_category"].strip()
        if source_category not in category_map:
            raise ValueError(f"Unknown category: {source_category}")

        category = category_map[source_category]
        eligible_count += 1

    elif record_type == "RETENTION_ONLY_REFERENCE":
        if source_eligible:
            raise ValueError(f"Unexpected eligible reference: {name}")

        source_category = row["auction_category"].strip()
        if source_category not in reference_category_map:
            raise ValueError(f"Unknown reference category: {source_category}")

        category = reference_category_map[source_category]
        reference_count += 1

    else:
        raise ValueError(f"Unknown record type: {record_type}")

    category_counts[category] += 1

    output_rows.append({
        "full_name": name,
        "auction_category": category,
        "is_auction_eligible": "true",
    })

if eligible_count != 413 or reference_count != 130:
    raise ValueError(
        f"Unexpected counts: eligible={eligible_count}, "
        f"reference={reference_count}"
    )

if len(output_rows) != 543:
    raise ValueError(f"Expected 543 output rows, found {len(output_rows)}")

with open(OUTPUT, "w", newline="", encoding="utf-8") as f:
    writer = csv.DictWriter(
        f,
        fieldnames=[
            "full_name",
            "auction_category",
            "is_auction_eligible",
        ],
    )
    writer.writeheader()
    writer.writerows(output_rows)

print("Metadata CSV generated:", OUTPUT)
print("Total:", len(output_rows))
print("Auction candidates:", eligible_count)
print("Reference players included:", reference_count)
print("Total auction eligible:", sum(
    1 for row in output_rows
    if row["is_auction_eligible"] == "true"
))
print("Categories:", dict(category_counts))
