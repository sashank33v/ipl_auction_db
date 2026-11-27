"""Read-only pre-import audit for the repository's fixed 543-player snapshot."""
import csv
from collections import Counter
from decimal import Decimal, InvalidOperation
from pathlib import Path

ROOT = Path(__file__).resolve().parent / "datasets"


def read_rows(name):
    with (ROOT / name).open(newline="", encoding="utf-8-sig") as f:
        return list(csv.DictReader(f))


def index_rows(rows, key):
    result = {}
    for row in rows:
        name = row[key].strip().casefold()
        if not name or name in result:
            raise ValueError(f"Blank or duplicate player identity: {row[key]!r}")
        result[name] = row
    return result


def audit():
    master = index_rows(read_rows("players_master_roles_updated.csv"), "player_name_official")
    candidates = index_rows(read_rows("ipl_auction_players_seed.csv"), "full_name")
    references = index_rows(read_rows("ipl_auction_retention_130_seed.csv"), "full_name")
    metadata = index_rows(read_rows("player_auction_metadata_543.csv"), "full_name")
    errors = []
    if (len(master), len(candidates), len(references), len(metadata)) != (543, 413, 130, 543):
        errors.append("Unexpected snapshot counts")
    if candidates.keys() & references.keys():
        errors.append("Candidate and reference seed files overlap")
    if master.keys() != candidates.keys() | references.keys() or master.keys() != metadata.keys():
        errors.append("Player identities differ across files")
    counts = Counter()
    from generate_metadata import category
    unified = index_rows(read_rows("ipl_players_unified_543.csv"), "full_name")
    if unified.keys() != master.keys():
        errors.append("Unified dataset identity mismatch")
    for key, source in master.items():
        name = source["player_name_official"]
        eligible = source["auction_eligible_in_this_dataset"].strip().lower()
        source_type = source["dataset_record_type"]
        if eligible != "true" or source_type not in {"AUCTION_CANDIDATE", "RETENTION_ONLY_REFERENCE"}:
            errors.append(f"{name}: invalid fantasy eligibility or provenance")
            continue
        counts[source_type] += 1
        expected_category = category(source)
        if key not in metadata or metadata[key]["is_auction_eligible"] != "true" or metadata[key]["auction_category"] != expected_category:
            errors.append(f"{name}: incorrect metadata eligibility/category")
        seed = (references if source_type == "RETENTION_ONLY_REFERENCE" else candidates).get(key)
        u = unified.get(key)
        if u is None or u["auction_category"] != expected_category or u["is_auction_eligible"] != "true" or (seed is not None and any(u[k] != v for k,v in seed.items())):
            errors.append(f"{name}: incorrect unified record")
        if seed is None:
            errors.append(f"{name}: missing player seed")
            continue
        country = seed["country"].strip()
        if not country or country.upper() == "UNVERIFIED" or country != source["country"].strip():
            errors.append(f"{name}: player nationality unverified or inconsistent")
        elif seed["is_overseas"].strip().lower() != str(country.casefold() != "india").lower():
            errors.append(f"{name}: incorrect overseas classification")
        role = source["specialism"].upper().replace("-", "_").replace("WICKETKEEPER", "WICKET_KEEPER")
        if role not in {"BATTER", "BOWLER", "ALL_ROUNDER", "WICKET_KEEPER"} or seed["role"] != role:
            errors.append(f"{name}: invalid or inconsistent role")
        try:
            price = Decimal(seed["base_price"])
            if not price.is_finite() or price <= 0 or price != Decimal(source["effective_fantasy_base_price_cr"]):
                errors.append(f"{name}: invalid or inconsistent base price")
        except InvalidOperation:
            errors.append(f"{name}: invalid base price")
    print(f"Original candidates: {counts['AUCTION_CANDIDATE']}; retained-source records: {counts['RETENTION_ONLY_REFERENCE']}; eligible: {sum(counts.values())}")
    for error in errors:
        print(f"ERROR: {error}")
    print(f"Import readiness: {'BLOCKED' if errors else 'PASS'} ({len(errors)} issues)")
    return errors


if __name__ == "__main__":
    raise SystemExit(1 if audit() else 0)
