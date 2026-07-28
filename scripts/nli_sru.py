#!/usr/bin/env python3
"""Fetch and normalize NLI manuscript MARCXML through Alma SRU.

This uses the public SRU endpoint and does not require an API key.
"""

from __future__ import annotations

import argparse
import csv
import hashlib
import json
import sys
import time
import urllib.parse
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable
from xml.etree import ElementTree as ET


SRU_BASE_URL = "https://nli.alma.exlibrisgroup.com/view/sru/972NNL_INST"
MARC_NS = "http://www.loc.gov/MARC21/slim"
SRU_NS = "http://www.loc.gov/zing/srw/"
DEFAULT_CSV_COLUMN = "מספר מערכת"


@dataclass(frozen=True)
class Subfield:
    code: str
    value: str


@dataclass(frozen=True)
class Field:
    tag: str
    ind1: str
    ind2: str
    subfields: tuple[Subfield, ...]

    def values(self, code: str) -> list[str]:
        return [s.value for s in self.subfields if s.code == code]

    def first(self, code: str) -> str | None:
        values = self.values(code)
        return values[0] if values else None


@dataclass(frozen=True)
class MarcRecord:
    leader: str
    controlfields: dict[str, tuple[str, ...]]
    fields: tuple[Field, ...]

    @property
    def mms_id(self) -> str:
        values = self.controlfields.get("001", ())
        return values[0] if values else ""

    def tagged(self, tag: str) -> list[Field]:
        return [field for field in self.fields if field.tag == tag]


def build_sru_url(query: str, maximum_records: int = 50) -> str:
    params = {
        "version": "1.2",
        "operation": "searchRetrieve",
        "recordSchema": "marcxml",
        "maximumRecords": str(maximum_records),
        "query": query,
    }
    return f"{SRU_BASE_URL}?{urllib.parse.urlencode(params)}"


def fetch_url(url: str, timeout: float = 30.0) -> bytes:
    request = urllib.request.Request(
        url,
        headers={
            "Accept": "application/xml,text/xml;q=0.9,*/*;q=0.1",
            "User-Agent": "midrash-atlas-poc/0.1",
        },
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return response.read()


def parse_sru_records(payload: bytes) -> list[MarcRecord]:
    root = ET.fromstring(payload)
    records: list[MarcRecord] = []
    for record_data in root.findall(f".//{{{SRU_NS}}}recordData"):
        record_el = record_data.find(f"{{{MARC_NS}}}record")
        if record_el is None:
            continue
        leader = (record_el.findtext(f"{{{MARC_NS}}}leader") or "").strip()
        controls: dict[str, list[str]] = {}
        for element in record_el.findall(f"{{{MARC_NS}}}controlfield"):
            controls.setdefault(element.attrib.get("tag", ""), []).append(
                (element.text or "").strip()
            )
        fields: list[Field] = []
        for element in record_el.findall(f"{{{MARC_NS}}}datafield"):
            subfields = tuple(
                Subfield(
                    code=sub.attrib.get("code", ""),
                    value="".join(sub.itertext()).strip(),
                )
                for sub in element.findall(f"{{{MARC_NS}}}subfield")
            )
            fields.append(
                Field(
                    tag=element.attrib.get("tag", ""),
                    ind1=element.attrib.get("ind1", " "),
                    ind2=element.attrib.get("ind2", " "),
                    subfields=subfields,
                )
            )
        records.append(
            MarcRecord(
                leader=leader,
                controlfields={key: tuple(values) for key, values in controls.items()},
                fields=tuple(fields),
            )
        )
    return records


def load_or_fetch(
    query: str,
    cache_path: Path | None,
    *,
    maximum_records: int = 50,
    timeout: float = 30.0,
) -> bytes:
    if cache_path is not None and cache_path.exists():
        return cache_path.read_bytes()
    payload = fetch_url(build_sru_url(query, maximum_records), timeout=timeout)
    if cache_path is not None:
        cache_path.parent.mkdir(parents=True, exist_ok=True)
        cache_path.write_bytes(payload)
    return payload


def fetch_record(
    mms_id: str, cache_dir: Path | None = None, timeout: float = 30.0
) -> MarcRecord | None:
    cache_path = cache_dir / f"{mms_id}.xml" if cache_dir else None
    payload = load_or_fetch(
        f"alma.mms_id={mms_id}", cache_path, maximum_records=1, timeout=timeout
    )
    records = parse_sru_records(payload)
    return records[0] if records else None


def fetch_records(
    mms_ids: list[str],
    cache_dir: Path | None = None,
    timeout: float = 30.0,
) -> list[MarcRecord]:
    if not mms_ids:
        return []
    query = " or ".join(f"alma.mms_id={mms_id}" for mms_id in mms_ids)
    if cache_dir:
        digest = hashlib.sha256("\n".join(mms_ids).encode()).hexdigest()[:16]
        cache_path = cache_dir / f"batch-{digest}.xml"
    else:
        cache_path = None
    payload = load_or_fetch(
        query,
        cache_path,
        maximum_records=len(mms_ids),
        timeout=timeout,
    )
    return parse_sru_records(payload)


def fetch_children(
    parent_mms_id: str,
    cache_dir: Path | None = None,
    timeout: float = 30.0,
) -> list[MarcRecord]:
    cache_path = (
        cache_dir / f"children-of-{parent_mms_id}.xml" if cache_dir else None
    )
    payload = load_or_fetch(
        f"alma.other_system_number={parent_mms_id}",
        cache_path,
        maximum_records=50,
        timeout=timeout,
    )
    records = parse_sru_records(payload)
    # The SRU index can contain identifiers from fields other than 773. Keep
    # only records that explicitly identify this manuscript as their host.
    return [
        record
        for record in records
        if parent_mms_id
        in {
            value
            for field in record.tagged("773")
            for value in field.values("w")
        }
    ]


def field_objects(fields: Iterable[Field]) -> list[dict[str, object]]:
    return [
        {
            "ind1": field.ind1,
            "ind2": field.ind2,
            "subfields": [
                {"code": subfield.code, "value": subfield.value}
                for subfield in field.subfields
            ],
        }
        for field in fields
    ]


def values(record: MarcRecord, tag: str, code: str) -> list[str]:
    return [
        value
        for field in record.tagged(tag)
        for value in field.values(code)
        if value
    ]


def owner_objects(record: MarcRecord) -> list[dict[str, object]]:
    owners = []
    for field in record.tagged("710"):
        roles = field.values("e")
        if not any(role in {"current owner", "בעלים נוכחיים"} for role in roles):
            continue
        owners.append(
            {
                "name": field.first("a"),
                "locality": field.first("x"),
                "country": field.first("c"),
                "roles": roles,
                "language": field.first("9"),
            }
        )
    return owners


def shelfmark_objects(record: MarcRecord) -> list[dict[str, object]]:
    objects = []
    for field in record.tagged("942"):
        if field.ind1 != "1":
            continue
        objects.append(
            {
                "repository": field.first("a"),
                "locality": field.first("x"),
                "country": field.first("c"),
                "shelfmark": field.first("z"),
                "language": field.first("9"),
            }
        )
    return objects


def place_objects(record: MarcRecord) -> list[dict[str, object]]:
    return [
        {
            "name": field.first("a"),
            "role": field.first("e"),
            "language": field.first("9"),
            "authority_id": field.first("0"),
        }
        for field in record.tagged("751")
    ]


def digital_object_objects(record: MarcRecord) -> list[dict[str, object]]:
    objects = []
    for field in record.tagged("907"):
        representation_id = field.first("c")
        file_id = field.first("d")
        objects.append(
            {
                "enabled": field.first("a"),
                "mms_id": field.first("b"),
                "representation_id": representation_id,
                "file_id": file_id,
                "label": field.first("e"),
                "media_type": field.first("g"),
                "loaded_date": field.first("h"),
                "page_count_hint": field.first("i"),
                "access_class": field.first("k"),
                "derivative_formats": field.first("o"),
                "sample_filename": field.first("n"),
                # SRU exposes the Rosetta IE and FL identifiers used by NLI's
                # Presentation and Image services. Treat these as candidates:
                # access policies can still make a particular request fail.
                "nli_manifest_url": (
                    f"https://iiif.nli.org.il/IIIFv21/{representation_id}/manifest"
                    if representation_id
                    else None
                ),
                "nli_image_info_url": (
                    f"https://iiif.nli.org.il/IIIFv21/{file_id}/info.json"
                    if file_id
                    else None
                ),
            }
        )
    return objects


def parent_ids(record: MarcRecord) -> list[str]:
    return list(
        dict.fromkeys(
            value
            for field in record.tagged("773")
            for value in field.values("w")
            if value
        )
    )


def resolver_url(mms_id: str, language: str = "eng") -> str:
    params = {
        "u.ignore_date_coverage": "true",
        "rft.mms_id": mms_id,
        "rfr_id": "info:sid/primo.exlibrisgroup.com",
        "svc_dat": "viewit",
        "is_new_ui": "true",
        "rft_dat": f"language={language}",
    }
    return (
        "https://nli.alma.exlibrisgroup.com/view/uresolver/"
        f"972NNL_INST/openurl?{urllib.parse.urlencode(params)}"
    )


def normalize_record(record: MarcRecord) -> dict[str, object]:
    mms_id = record.mms_id
    return {
        "mms_id": mms_id,
        "record_kind": "analytic_child" if parent_ids(record) else "manuscript",
        "parent_mms_ids": parent_ids(record),
        "title": values(record, "245", "a"),
        "alternative_titles": values(record, "740", "a"),
        "date_display": values(record, "260", "c"),
        "production_place_display": values(record, "260", "a"),
        "places": place_objects(record),
        "extent": values(record, "300", "a"),
        "languages": values(record, "041", "a"),
        "script_styles": values(record, "958", "a"),
        "physical_notes": values(record, "340", "a"),
        "contents": values(record, "505", "a"),
        "general_notes": values(record, "500", "a"),
        "provenance_notes": values(record, "561", "a"),
        "colophon_notes": values(record, "957", "a"),
        "current_owners": owner_objects(record),
        "shelfmarks": shelfmark_objects(record),
        "catalog_references": [
            {
                "catalog": field.first("a"),
                "entry": field.first("z"),
                "authority_id": field.first("0"),
            }
            for field in record.tagged("942")
            if field.ind1 == "3"
        ],
        "rights": field_objects(record.tagged("903")),
        "digital_services": field_objects(record.tagged("AVE")),
        "digital_objects": digital_object_objects(record),
        "sfardata_ids": [
            field.first("a")
            for field in record.tagged("024")
            if field.first("2") == "Sfardata" and field.first("a")
        ],
        "resolver_url": resolver_url(mms_id),
        "public_record_url": (
            f"https://www.nli.org.il/en/manuscripts/NNL_ALEPH{mms_id}/NLI"
        ),
        "source_modified": (record.controlfields.get("005") or (None,))[0],
        "fixed_field_008": (record.controlfields.get("008") or (None,))[0],
        "marc": {
            "leader": record.leader,
            "controlfields": record.controlfields,
            "fields": [
                {
                    "tag": field.tag,
                    "ind1": field.ind1,
                    "ind2": field.ind2,
                    "subfields": [
                        {"code": subfield.code, "value": subfield.value}
                        for subfield in field.subfields
                    ],
                }
                for field in record.fields
            ],
        },
    }


def ids_from_csv(path: Path, column: str) -> list[str]:
    with path.open(encoding="utf-8-sig", newline="") as handle:
        rows = csv.DictReader(handle)
        if not rows.fieldnames or column not in rows.fieldnames:
            raise ValueError(f"CSV column not found: {column}")
        identifiers = []
        for row in rows:
            value = (row.get(column) or "").strip()
            if value.isdigit():
                identifiers.append(value)
        return list(dict.fromkeys(identifiers))


def chunks(values_to_chunk: list[str], size: int) -> Iterable[list[str]]:
    for start in range(0, len(values_to_chunk), size):
        yield values_to_chunk[start : start + size]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mms_ids", nargs="*", help="NLI/Alma manuscript MMS IDs")
    parser.add_argument("--csv", type=Path, help="Read MMS IDs from a CSV file")
    parser.add_argument("--csv-column", default=DEFAULT_CSV_COLUMN)
    parser.add_argument("--limit", type=int)
    parser.add_argument(
        "--related",
        choices=("none", "parents", "children", "all"),
        default="none",
        help="Also retrieve directly related parent and/or child records",
    )
    parser.add_argument(
        "--cache-dir",
        type=Path,
        default=Path("data/raw/nli_sru"),
        help="Directory for cached SRU XML responses; use an empty value to disable",
    )
    parser.add_argument("--output", type=Path, help="Write JSON to this file")
    parser.add_argument("--timeout", type=float, default=30.0)
    parser.add_argument(
        "--batch-size",
        type=int,
        default=40,
        help="Number of exact MMS IDs per SRU request",
    )
    parser.add_argument(
        "--delay",
        type=float,
        default=0.25,
        help="Polite delay between uncached top-level requests",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    identifiers = list(args.mms_ids)
    if args.csv:
        identifiers.extend(ids_from_csv(args.csv, args.csv_column))
    identifiers = list(dict.fromkeys(identifiers))
    if args.limit is not None:
        identifiers = identifiers[: args.limit]
    if not identifiers:
        print("No MMS IDs supplied.", file=sys.stderr)
        return 2

    normalized: dict[str, dict[str, object]] = {}
    errors: list[dict[str, str]] = []

    def add_record_batch(mms_ids: list[str]) -> None:
        pending = [mms_id for mms_id in mms_ids if mms_id not in normalized]
        if not pending:
            return
        for batch in chunks(pending, args.batch_size):
            try:
                records = fetch_records(batch, args.cache_dir, timeout=args.timeout)
                found = {record.mms_id for record in records}
                for record in records:
                    normalized[record.mms_id] = normalize_record(record)
                for missing in (mms_id for mms_id in batch if mms_id not in found):
                    errors.append({"mms_id": missing, "error": "record not found"})
                if args.delay:
                    time.sleep(args.delay)
            except Exception as exc:
                for mms_id in batch:
                    errors.append({"mms_id": mms_id, "error": str(exc)})

    add_record_batch(identifiers)

    if args.related in {"parents", "all"}:
        # Parent chains are normally one level, but follow until no new parent
        # identifiers are found so the representation remains faithful.
        while True:
            parents = list(
                dict.fromkeys(
                    parent_id
                    for record in normalized.values()
                    for parent_id in record["parent_mms_ids"]
                    if parent_id not in normalized
                )
            )
            if not parents:
                break
            before = len(normalized)
            add_record_batch(parents)
            if len(normalized) == before:
                break

    if args.related in {"children", "all"}:
        parent_ids_to_scan = [
            mms_id
            for mms_id, record in normalized.items()
            if record["record_kind"] == "manuscript"
        ]
        for mms_id in parent_ids_to_scan:
            try:
                for child in fetch_children(
                    mms_id, args.cache_dir, timeout=args.timeout
                ):
                    normalized.setdefault(child.mms_id, normalize_record(child))
                if args.delay:
                    time.sleep(args.delay)
            except Exception as exc:
                errors.append(
                    {"mms_id": mms_id, "error": f"child lookup failed: {exc}"}
                )

    # Preserve requested order, followed by records discovered via relations.
    ordered_ids = list(
        dict.fromkeys(identifiers + [mms_id for mms_id in normalized])
    )
    ordered_records = [
        normalized[mms_id] for mms_id in ordered_ids if mms_id in normalized
    ]

    result = {
        "source": {
            "service": "NLI Alma SRU",
            "base_url": SRU_BASE_URL,
            "record_schema": "marcxml",
        },
        "requested_mms_ids": identifiers,
        "records": ordered_records,
        "errors": errors,
    }
    text = json.dumps(result, ensure_ascii=False, indent=2)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(text + "\n", encoding="utf-8")
    else:
        print(text)
    return 0 if not errors else 1


if __name__ == "__main__":
    raise SystemExit(main())
