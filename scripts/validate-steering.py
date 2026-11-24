#!/usr/bin/env python3
"""Validate the persisted development-cycle registry and decision records using Python stdlib."""
from __future__ import annotations
import json, re, sys, subprocess
from datetime import datetime
from pathlib import Path
from uuid import UUID

ROOT = Path(__file__).resolve().parents[1]
REGISTRY = ROOT / "docs/dev-life-cycle/cycle-index.json"
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

def validate_decision_register(path):
    """Require each persisted material decision to be proposed or fully recorded."""
    try:
        content = path.read_text(encoding="utf-8")
    except OSError as e:
        fail(f"{path.relative_to(ROOT)}: cannot read handoff: {e}")
    if "### Decision register" not in content:
        fail(f"{path.relative_to(ROOT)}: missing Decision register")
    lines = content.splitlines()
    header = "| ID | Decision | State (Proposed / Approved / Recorded) | Canonical artifact | Verified revision / evidence |"
    try:
        start = lines.index(header) + 2
    except ValueError:
        fail(f"{path.relative_to(ROOT)}: missing or malformed decision-register table")
    for line in lines[start:]:
        if not line.startswith("|"):
            break
        cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
        if len(cells) != 5:
            fail(f"{path.relative_to(ROOT)}: decision-register rows must have five columns")
        decision_id, decision, state, artifact, evidence = cells
        if decision_id.startswith("<") or decision.startswith("<"):
            continue
        if state not in {"Proposed", "Recorded"}:
            fail(f"{path.relative_to(ROOT)}: {decision_id} is {state}; approved decisions must be recorded before checkpointing or continuing")
        if state == "Recorded":
            if not artifact or artifact in {"—", "-", "<Path or —>"} or artifact.startswith("<"):
                fail(f"{path.relative_to(ROOT)}: {decision_id} is Recorded without a canonical artifact path")
            if not evidence or not re.search(r"\b[0-9a-f]{7,40}\b", evidence):
                fail(f"{path.relative_to(ROOT)}: {decision_id} is Recorded without a verified commit/revision")
            revision_match = re.search(r"\b([0-9a-f]{7,40})\b", evidence)
            revision = revision_match.group(1)
            artifact_path = Path(artifact)
            if artifact_path.is_absolute() or ".." in artifact_path.parts:
                fail(f"{path.relative_to(ROOT)}: {decision_id} has an unsafe canonical artifact path")
            # Verify the revision is a real commit reachable from this checkout and contains the artifact.
            try:
                subprocess.run(["git", "cat-file", "-e", f"{revision}^{{commit}}"], cwd=ROOT, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                subprocess.run(["git", "merge-base", "--is-ancestor", revision, "HEAD"], cwd=ROOT, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                subprocess.run(["git", "cat-file", "-e", f"{revision}:{artifact}"], cwd=ROOT, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            except (subprocess.CalledProcessError, FileNotFoundError):
                fail(f"{path.relative_to(ROOT)}: {decision_id} revision {revision} is not a verified ancestor containing {artifact}")
            if not (ROOT / artifact_path).is_file():
                fail(f"{path.relative_to(ROOT)}: {decision_id} references missing canonical artifact {artifact}")
def validate_registry(data):
    if not isinstance(data, dict) or set(data)!={"schema_version","docs_reviewed_through_execution_order","cycles"} or data.get("schema_version") != 3: fail("registry: exact root keys and schema_version 3 required")
    watermark = data.get("docs_reviewed_through_execution_order")
    if not isinstance(watermark, int) or isinstance(watermark, bool) or watermark < 0: fail("registry: documentation watermark must be >= 0")
    cycles = data.get("cycles")
    if not isinstance(cycles, list): fail("registry: cycles must be an array")
    ids, seqs, orders = set(), set(), set()
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
            validate_decision_register(cycle_dir / "handoff/HANDOFF.md")
            if any(c.get(f) is not None for f in ("eligibility_reason","acceptance_criteria","verification_evidence","outcome")): fail(f"{label}: micro-only fields must be null for standard cycles")
            if st=="pending" and (ph!="assess" or o is not None or started is not None or completed is not None or c.get("status_reason") is not None): fail(f"{label}: pending cycle must be assess and have null order/timestamps/reason")
            if st in {"in-progress","paused","blocked"} and (o is None or started is None or completed is not None or ph=="complete"): fail(f"{label}: invalid started/paused/blocked standard-cycle state")
            if st=="in-progress" and c.get("status_reason") is not None: fail(f"{label}: active cycle must have null status_reason")
            if st in {"blocked","cancelled"} and not c.get("status_reason"): fail(f"{label}: blocked/cancelled requires status_reason")
            if st=="paused" and c.get("status_reason") is not None: fail(f"{label}: paused cycle reason belongs in handoff, status_reason must be null")
            if st=="cancelled" and ((o is None) != (started is None) or completed is not None): fail(f"{label}: cancelled cycles retain either both start fields or neither; completed_at remains null")
            if st=="complete" and (o is None or started is None or completed is None or ph!="complete" or c.get("status_reason") is not None): fail(f"{label}: invalid completed standard-cycle state")
            if st=="blocked" and not c.get("status_reason"): fail(f"{label}: blocked cycle requires reason")
        else:
            if st not in {"in-progress","complete"} or o is None or started is None: fail(f"{label}: micro cycles require started state/order; promote to standard to block/cancel")
            if created is None or not c.get("eligibility_reason") or not c.get("acceptance_criteria"): fail(f"{label}: micro cycles require created_at, eligibility, and acceptance criteria")
            if c.get("status_reason") is not None: fail(f"{label}: micro status_reason must be null; promote to standard for interruption")
            if st=="in-progress":
                if ph!="implement" or completed is not None or c.get("verification_evidence") is not None or c.get("outcome") is not None: fail(f"{label}: invalid active micro-cycle state")
            elif ph!="complete" or completed is None or not c.get("verification_evidence") or not c.get("outcome"): fail(f"{label}: completed micro-cycle evidence missing")

def main():
    validate_decision_register(ROOT / "docs/dev-life-cycle/templates/HANDOFF.md")
    registry=read_json(REGISTRY)
    validate_registry(registry)
    print("Steering cycle registry and decision records: valid")
if __name__=="__main__":
    try: main()
    except ValueError as e:
        print(f"ERROR: {e}",file=sys.stderr); sys.exit(1)
