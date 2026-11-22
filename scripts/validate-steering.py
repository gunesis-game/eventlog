#!/usr/bin/env python3
"""Validate the persisted development-cycle registry and lock using Python stdlib."""
from __future__ import annotations
import json, sys
from datetime import datetime
from pathlib import Path
from uuid import UUID

ROOT = Path(__file__).resolve().parents[1]
REGISTRY = ROOT / "docs/dev-life-cycle/cycle-index.json"
LOCK = ROOT / "docs/dev-life-cycle/execution-lock.json"
PHASES = {"assess", "define", "implement", "verify", "reconcile", "complete"}
MICRO_PHASES = {"implement", "complete"}
STATUSES = {"pending", "in-progress", "paused", "blocked", "cancelled", "complete"}
WORKFLOWS = {"standard", "micro"}

def fail(message):
    raise ValueError(message)

def read_json(path):
    try: return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as e: fail(f"{path.relative_to(ROOT)}: invalid/unreadable JSON: {e}")

def parse_time(value, label, nullable=True):
    if value is None and nullable: return None
    if not isinstance(value, str) or not value.strip(): fail(f"{label}: expected ISO-8601 timestamp or null")
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
        if parsed.tzinfo is None: fail(f"{label}: timezone offset required")
        return parsed
    except ValueError: fail(f"{label}: invalid ISO-8601 timestamp {value!r}")

def validate_registry(data):
    if not isinstance(data, dict) or set(data)!={"schema_version","docs_reviewed_through_execution_order","cycles"} or data.get("schema_version") != 3: fail("registry: exact root keys and schema_version 3 required")
    watermark = data.get("docs_reviewed_through_execution_order")
    if not isinstance(watermark, int) or isinstance(watermark, bool) or watermark < 0: fail("registry: documentation watermark must be >= 0")
    cycles = data.get("cycles")
    if not isinstance(cycles, list): fail("registry: cycles must be an array")
    ids, seqs, orders, running = set(), set(), set(), []
    for n, c in enumerate(cycles):
        label=f"cycles[{n}]"
        if not isinstance(c, dict): fail(f"{label}: expected object")
        required={"id","sequence","execution_order","workflow","description","status","created_at","started_at","completed_at","current_phase","status_reason","eligibility_reason","acceptance_criteria","verification_evidence","outcome"}
        if set(c)!=required: fail(f"{label}: exact schema keys required (missing/extra fields)")
        if not isinstance(c.get("description"), str) or not c["description"].strip(): fail(f"{label}.description: non-empty string required")
        try: cid=str(UUID(c.get("id","")))
        except (ValueError, AttributeError): fail(f"{label}.id: must be UUID")
        if cid in ids: fail(f"{label}.id: duplicate")
        ids.add(cid)
        s=c.get("sequence")
        if not isinstance(s,int) or isinstance(s,bool) or s < 1 or s in seqs: fail(f"{label}.sequence: unique positive integer required")
        seqs.add(s)
        o=c.get("execution_order")
        if o is not None and (not isinstance(o,int) or isinstance(o,bool) or o < 1 or o in orders): fail(f"{label}.execution_order: unique positive integer or null required")
        if o is not None: orders.add(o)
        w, st, ph = c.get("workflow"), c.get("status"), c.get("current_phase")
        if w not in WORKFLOWS or st not in STATUSES or ph not in (MICRO_PHASES if w=="micro" else PHASES): fail(f"{label}: invalid workflow/status/phase")
        created=parse_time(c.get("created_at"), f"{label}.created_at")
        started=parse_time(c.get("started_at"), f"{label}.started_at")
        completed=parse_time(c.get("completed_at"), f"{label}.completed_at")
        for f in ("description","status_reason","eligibility_reason","acceptance_criteria","verification_evidence","outcome"):
            v=c.get(f)
            if v is not None and (not isinstance(v,str) or not v.strip()): fail(f"{label}.{f}: non-empty string or null required")
        if created and started and created > started: fail(f"{label}: created_at after started_at")
        if started and completed and started > completed: fail(f"{label}: started_at after completed_at")
        if w=="standard":
            cycle_dir=ROOT / "docs/dev-life-cycle" / cid
            if not (cycle_dir / "plan/PLAN.md").is_file() or not (cycle_dir / "handoff/HANDOFF.md").is_file(): fail(f"{label}: standard cycle requires plan and handoff files")
            if any(c.get(f) is not None for f in ("eligibility_reason","acceptance_criteria","verification_evidence","outcome")): fail(f"{label}: micro-only fields must be null for standard cycles")
            if st=="pending" and (ph!="assess" or o is not None or started is not None or completed is not None or c.get("status_reason") is not None): fail(f"{label}: pending cycle must be assess and have null order/timestamps/reason")
            if st in {"in-progress","paused","blocked"} and (o is None or started is None or completed is not None or ph=="complete"): fail(f"{label}: invalid started/paused/blocked standard-cycle state")
            if st=="in-progress" and c.get("status_reason") is not None: fail(f"{label}: active cycle must have null status_reason")
            if st in {"blocked","cancelled"} and not c.get("status_reason"): fail(f"{label}: blocked/cancelled requires status_reason")
            if st=="paused" and c.get("status_reason") is not None: fail(f"{label}: paused cycle reason belongs in handoff, status_reason must be null")
            if st=="cancelled" and ((o is None) != (started is None) or completed is not None): fail(f"{label}: cancelled cycles retain either both start fields or neither; completed_at remains null")
            if st=="complete" and (o is None or started is None or completed is None or ph!="complete" or c.get("status_reason") is not None): fail(f"{label}: invalid completed standard-cycle state")
            if st=="in-progress": running.append(c)
            if st=="blocked" and not c.get("status_reason"): fail(f"{label}: blocked cycle requires reason")
        else:
            if st not in {"in-progress","complete"} or o is None or started is None: fail(f"{label}: micro cycles require started state/order; promote to standard to block/cancel")
            if created is None or not c.get("eligibility_reason") or not c.get("acceptance_criteria"): fail(f"{label}: micro cycles require created_at, eligibility, and acceptance criteria")
            if c.get("status_reason") is not None: fail(f"{label}: micro status_reason must be null; promote to standard for interruption")
            if st=="in-progress":
                if ph!="implement" or completed is not None or c.get("verification_evidence") is not None or c.get("outcome") is not None: fail(f"{label}: invalid active micro-cycle state")
                running.append(c)
            elif ph!="complete" or completed is None or not c.get("verification_evidence") or not c.get("outcome"): fail(f"{label}: completed micro-cycle evidence missing")
    if len(running)>1: fail("registry: multiple in-progress cycles")

def validate_lock(lock, cycles):
    if not isinstance(lock,dict) or set(lock)!={"schema_version","status","cycle_id","workflow","worker_id","claimed_at","released_at","release_evidence"} or lock.get("schema_version")!=1: fail("execution lock: exact keys and schema_version 1 required")
    st=lock.get("status")
    if st not in {"released","active"}: fail("execution lock: invalid status")
    fields=("cycle_id","workflow","worker_id","claimed_at")
    if st=="released":
        if any(lock.get(k) is not None for k in fields): fail("execution lock: released state must clear owner fields")
        ev=lock.get("release_evidence")
        if ev is not None and (not isinstance(ev,str) or not ev.strip()): fail("execution lock: release evidence must be non-empty or null")
        parse_time(lock.get("released_at"),"execution lock.released_at")
        return
    if not lock.get("cycle_id") or lock.get("workflow") not in WORKFLOWS or not isinstance(lock.get("worker_id"),str) or not lock["worker_id"].strip(): fail("execution lock: active owner fields required")
    parse_time(lock.get("claimed_at"),"execution lock.claimed_at",False)
    if lock.get("released_at") is not None or lock.get("release_evidence") is not None: fail("execution lock: active lock must have null release fields")
    found=[c for c in cycles if c.get("id")==lock["cycle_id"]]
    if len(found)!=1 or found[0].get("workflow")!=lock["workflow"] or found[0].get("status")!="in-progress": fail("execution lock: must match exactly one in-progress registry cycle")

def main():
    registry=read_json(REGISTRY); lock=read_json(LOCK)
    validate_registry(registry); validate_lock(lock,registry["cycles"])
    print("Steering registry and execution lock: valid")
if __name__=="__main__":
    try: main()
    except ValueError as e:
        print(f"ERROR: {e}",file=sys.stderr); sys.exit(1)
