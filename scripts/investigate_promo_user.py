#!/usr/bin/env python3
import argparse
import datetime as dt
import json
import os
import re
import sys

try:
    import pymysql
except ImportError:  # pragma: no cover
    pymysql = None


def load_env(path):
    env = {}
    if not os.path.exists(path):
        return env
    with open(path, "r", encoding="utf-8") as handle:
        for raw in handle:
            line = raw.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, value = line.split("=", 1)
            env[key.strip()] = value.strip().strip('"')
    return env


def parse_dsn(dsn):
    if "@tcp(" not in dsn:
        raise ValueError("Unsupported DSN format")
    prefix, rest = dsn.split("@tcp(", 1)
    user, password = prefix.split(":", 1)
    host_port, db_part = rest.split(")/", 1)
    host, port_raw = host_port.split(":", 1)
    db_name, _, params = db_part.partition("?")
    return {
        "user": user,
        "password": password,
        "host": host,
        "port": int(port_raw),
        "db": db_name,
        "params": params,
    }


def connect_mysql(dsn):
    if pymysql is None:
        print("pymysql is required. Install with: pip install pymysql", file=sys.stderr)
        sys.exit(1)
    config = parse_dsn(dsn)
    return pymysql.connect(
        host=config["host"],
        user=config["user"],
        password=config["password"],
        database=config["db"],
        port=config["port"],
        charset="utf8mb4",
        cursorclass=pymysql.cursors.DictCursor,
        autocommit=True,
    )


def fetch_one(conn, sql, params=None):
    with conn.cursor() as cur:
        cur.execute(sql, params or [])
        return cur.fetchone()


def fetch_all(conn, sql, params=None):
    with conn.cursor() as cur:
        cur.execute(sql, params or [])
        return cur.fetchall()


def parse_json_list(raw):
    if raw is None:
        return []
    try:
        data = json.loads(raw)
    except Exception:
        return []
    if isinstance(data, list):
        return [str(item).strip() for item in data if str(item).strip()]
    return []


def start_of_day_utc(now):
    return now.replace(hour=0, minute=0, second=0, microsecond=0)


def describe_event_weight(event_type):
    key = (event_type or "").lower()
    if key == "post_open":
        return "One time when a post is opened."
    if key == "scroll_depth":
        return "Fires at 25/50/75/90% scroll milestones."
    if key == "time_on_page":
        return "Fires at 15/45/90 seconds."
    if key == "post_complete":
        return "Triggered after scroll + time completion."
    if key == "promo_click":
        return "CTA clicks on promos."
    if key == "paywall_hit":
        return "Locked content attempts."
    return "Event contribution to funnel score."


def explain_promo(promo, decision, context):
    reasons = []
    now = context["now"]

    if context["post_access"] != "TRIAL":
        reasons.append(f"Post access level is {context['post_access']}, promos only show on TRIAL.")

    if context["entitlement_active"]:
        reasons.append("User has ACTIVE entitlement; promos suppressed for paid users.")

    if promo["status"].upper() != "ACTIVE":
        reasons.append(f"Promo status is {promo['status']} (not ACTIVE).")

    if promo["slot"].upper() != decision["slot"].upper():
        reasons.append(f"Slot mismatch: promo slot {promo['slot']} vs decision slot {decision['slot']}.")

    stages = parse_json_list(promo.get("eligible_stages_json"))
    if not stages:
        reasons.append("Eligible stages list is empty or invalid JSON.")
    else:
        stage_match = any(stage.lower() == context["stage"].lower() for stage in stages)
        if not stage_match:
            reasons.append(f"User stage {context['stage']} not in eligible stages {stages}.")

    start_at = promo.get("start_at")
    end_at = promo.get("end_at")
    if start_at and now < start_at:
        reasons.append(f"Promo not started yet (starts {start_at}).")
    if end_at and now > end_at:
        reasons.append(f"Promo expired (ended {end_at}).")

    metrics = context["promo_metrics"].get(promo["id"], {})
    impressions_today = metrics.get("impressions", 0)
    clicks_today = metrics.get("clicks", 0)
    last_click_at = metrics.get("last_click_at")

    if promo.get("max_impressions_per_day", 0) > 0 and impressions_today >= promo["max_impressions_per_day"]:
        reasons.append("Max impressions per day reached.")

    if promo.get("max_clicks_per_day", 0) > 0 and clicks_today >= promo["max_clicks_per_day"]:
        reasons.append("Max clicks per day reached.")

    cooldown_hours = promo.get("cooldown_hours", 0)
    if cooldown_hours and last_click_at:
        cooldown_until = last_click_at + dt.timedelta(hours=cooldown_hours)
        if now <= cooldown_until:
            reasons.append(f"Cooldown active until {cooldown_until}.")

    if context["variant_counts"].get(promo["id"], 0) == 0:
        reasons.append("Promo has no variants.")

    return reasons


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--user", type=int, default=2, help="User ID to investigate")
    parser.add_argument("--promo-code", default="ENGAGEED_TRIAL_CTA", help="Promo code to focus on")
    args = parser.parse_args()

    env = load_env(os.path.join(os.getcwd(), ".env"))
    dsn = env.get("DB_DSN") or env.get("DATABASE_DSN")
    if not dsn:
        print("DB_DSN not found in .env", file=sys.stderr)
        sys.exit(1)

    conn = connect_mysql(dsn)
    now = dt.datetime.utcnow()
    day_start = start_of_day_utc(now)

    print(f"Now (UTC): {now}")

    user = fetch_one(conn, "SELECT id, email, name, role, status FROM users WHERE id = %s", [args.user])
    print("\nUser:", user or "not found")

    metrics = fetch_one(conn, "SELECT user_id, score, stage, last_active_at, reads14d, completes14d FROM user_metrics WHERE user_id = %s", [args.user])
    print("User metrics:", metrics or "none")

    ent_active = fetch_one(
        conn,
        "SELECT id, status, plan_code, end_at FROM entitlements WHERE user_id = %s AND status = 'ACTIVE' AND end_at >= UTC_TIMESTAMP() ORDER BY end_at DESC LIMIT 1",
        [args.user],
    )
    print("Active entitlement:", ent_active or "none")

    anon_rows = fetch_all(
        conn,
        "SELECT anon_id, MAX(created_at) AS last_seen FROM events WHERE user_id = %s AND anon_id IS NOT NULL GROUP BY anon_id ORDER BY last_seen DESC LIMIT 5",
        [args.user],
    )
    anon_ids = [row["anon_id"] for row in anon_rows]
    if anon_ids:
        print("Recent anon_ids:", anon_ids)

    decisions = fetch_all(conn, "SELECT * FROM promo_decisions WHERE user_id = %s ORDER BY created_at DESC LIMIT 20", [args.user])
    for anon_id in anon_ids:
        decisions += fetch_all(conn, "SELECT * FROM promo_decisions WHERE anon_id = %s ORDER BY created_at DESC LIMIT 20", [anon_id])

    if not decisions:
        print("\nNo promo decisions found for user or anon ids.")
        return

    seen = set()
    unique_decisions = []
    for row in decisions:
        if row["decision_id"] in seen:
            continue
        seen.add(row["decision_id"])
        unique_decisions.append(row)

    print(f"\nPromo decisions ({len(unique_decisions)}):")
    for row in unique_decisions:
        print(f"- {row['decision_id']} slot={row['slot']} promo_id={row.get('promo_id')} variant_id={row.get('variant_id')} post_id={row.get('post_id')} created_at={row.get('created_at')}")

    promo_focus = fetch_all(conn, "SELECT * FROM promos WHERE code = %s", [args.promo_code])
    if promo_focus:
        print(f"\nPromo {args.promo_code}:")
        print(promo_focus[0])

    for decision in unique_decisions:
        post = None
        post_access = "UNKNOWN"
        if decision.get("post_id"):
            post = fetch_one(conn, "SELECT id, slug, access_level FROM posts WHERE id = %s", [decision["post_id"]])
            if post:
                post_access = post.get("access_level") or "UNKNOWN"

        stage = (metrics.get("stage") if metrics else "NEW") or "NEW"
        entitlement_active = ent_active is not None

        promos = fetch_all(conn, "SELECT * FROM promos WHERE slot = %s", [decision["slot"]])
        if not promos:
            print(f"\nDecision {decision['decision_id']} -> no promos found for slot {decision['slot']}")
            continue

        promo_ids = [promo["id"] for promo in promos]
        variant_counts = {}
        if promo_ids:
            rows = fetch_all(conn, "SELECT promo_id, COUNT(*) AS cnt FROM promo_variants WHERE promo_id IN %s GROUP BY promo_id", [promo_ids])
            for row in rows:
                variant_counts[row["promo_id"]] = row["cnt"]

        promo_metrics = {}
        for promo_id in promo_ids:
            impressions = fetch_one(
                conn,
                "SELECT COUNT(*) AS cnt FROM promo_impressions WHERE promo_id = %s AND created_at >= %s AND (user_id = %s OR (user_id IS NULL AND anon_id = %s))",
                [promo_id, day_start, decision.get("user_id"), decision.get("anon_id")],
            )
            clicks = fetch_one(
                conn,
                "SELECT COUNT(*) AS cnt FROM promo_clicks WHERE promo_id = %s AND created_at >= %s AND (user_id = %s OR (user_id IS NULL AND anon_id = %s))",
                [promo_id, day_start, decision.get("user_id"), decision.get("anon_id")],
            )
            last_click = fetch_one(
                conn,
                "SELECT MAX(created_at) AS last_click_at FROM promo_clicks WHERE promo_id = %s AND (user_id = %s OR (user_id IS NULL AND anon_id = %s))",
                [promo_id, decision.get("user_id"), decision.get("anon_id")],
            )
            promo_metrics[promo_id] = {
                "impressions": impressions["cnt"] if impressions else 0,
                "clicks": clicks["cnt"] if clicks else 0,
                "last_click_at": last_click["last_click_at"] if last_click else None,
            }

        context = {
            "now": now,
            "post_access": post_access,
            "stage": stage,
            "entitlement_active": entitlement_active,
            "promo_metrics": promo_metrics,
            "variant_counts": variant_counts,
        }

        print(f"\nDecision {decision['decision_id']} context:")
        print(f"  post_access={post_access}, stage={stage}, entitlement_active={entitlement_active}")
        if post:
            print(f"  post_slug={post.get('slug')}")

        for promo in promos:
            reasons = explain_promo(promo, decision, context)
            label = f"promo {promo['code']} (id {promo['id']})"
            if not reasons:
                print(f"  OK {label} eligible")
            else:
                print(f"  BLOCKED {label}: {', '.join(reasons)}")

    conn.close()


if __name__ == "__main__":
    main()
