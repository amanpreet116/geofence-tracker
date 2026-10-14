#!/usr/bin/env bash
# Verifies the API contract in docs/ASSIGNMENT.md against a running backend.
#
#   BASE_URL=http://localhost:8080 scripts/verify_contract.sh
#   BASE_URL=https://api.example.com scripts/verify_contract.sh
#
# Checks every specified endpoint (exact paths and methods, required fields
# and JSON types, time_ns as a numeric string on success AND error), the
# validation/404/409 error paths, and an end-to-end scenario including a
# WebSocket listener (websocat if installed, otherwise `npx wscat`).
# Creates its own uniquely named data. Prints a PASS/FAIL table and exits 1
# if anything failed.
#
# Requires: bash, curl, jq; websocat or node/npx for the WebSocket check.

set -uo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
BASE_URL="${BASE_URL%/}"
WS_URL="${WS_URL:-$(sed -e 's#^https://#wss://#' -e 's#^http://#ws://#' <<<"$BASE_URL")/ws/alerts}"
RUN_ID="$(date +%s)$RANDOM"
TMP="$(mktemp -d)"
trap 'kill "${WS_PID:-}" 2>/dev/null; rm -rf "$TMP"' EXIT

for tool in curl jq; do
  command -v "$tool" >/dev/null || { echo "missing dependency: $tool" >&2; exit 2; }
done

# ---------------------------------------------------------------- helpers ----

RESULTS=()  # "STATUS|name|detail"
PASS=0
FAIL=0

record() { # status name detail
  RESULTS+=("$1|$2|$3")
  if [[ $1 == PASS ]]; then PASS=$((PASS + 1)); else FAIL=$((FAIL + 1)); fi
}

# req METHOD PATH [JSON_BODY] -> sets STATUS and BODY
req() {
  local method=$1 path=$2 body=${3:-}
  local args=(-s -o "$TMP/body" -w '%{http_code}' -X "$method" "$BASE_URL$path")
  [[ -n $body ]] && args+=(-H 'Content-Type: application/json' --data "$body")
  STATUS=$(curl "${args[@]}" 2>/dev/null) || STATUS=000
  BODY=$(cat "$TMP/body" 2>/dev/null || true)
}

# check NAME EXPECTED_STATUS JQ_FILTER  (filter must evaluate to true)
# Always also checks that time_ns is a numeric string.
check() {
  local name=$1 want=$2 filter=$3
  if [[ $STATUS != "$want" ]]; then
    record FAIL "$name" "HTTP $STATUS, want $want: $(head -c 160 <<<"$BODY")"
    return 1
  fi
  if ! jq -e '(.time_ns | type == "string") and (.time_ns | test("^[0-9]+$"))' <<<"$BODY" >/dev/null 2>&1; then
    record FAIL "$name" "time_ns missing or not a numeric string: $(head -c 160 <<<"$BODY")"
    return 1
  fi
  if ! jq -e "$filter" <<<"$BODY" >/dev/null 2>&1; then
    record FAIL "$name" "shape check failed ($filter): $(head -c 160 <<<"$BODY")"
    return 1
  fi
  record PASS "$name" "HTTP $STATUS"
}

# Error responses: status + {"error": string, "time_ns": numeric string}.
check_error() { # name expected_status [extra jq filter]
  check "$1" "$2" "(.error | type == \"string\" and length > 0)${3:+ and ($3)}"
}

now_minus() { date -u -d "@$(($(date +%s) - $1))" +%Y-%m-%dT%H:%M:%SZ; }

# A small square at a random remote spot so repeated runs never overlap.
LAT0=$(awk -v r="$RANDOM" 'BEGIN { printf "%.4f", -40 - (r % 1000) / 100 }')
LNG0=$(awk -v r="$RANDOM" 'BEGIN { printf "%.4f", 140 + (r % 1000) / 100 }')
LAT1=$(awk -v a="$LAT0" 'BEGIN { printf "%.4f", a + 0.01 }')
LNG1=$(awk -v a="$LNG0" 'BEGIN { printf "%.4f", a + 0.01 }')
IN_LAT=$(awk -v a="$LAT0" 'BEGIN { printf "%.4f", a + 0.005 }')
IN_LNG=$(awk -v a="$LNG0" 'BEGIN { printf "%.4f", a + 0.005 }')
OUT_LAT=$(awk -v a="$LAT0" 'BEGIN { printf "%.4f", a - 0.02 }')
SQUARE="[[$LAT0,$LNG0],[$LAT1,$LNG0],[$LAT1,$LNG1],[$LAT0,$LNG1],[$LAT0,$LNG0]]"

echo "Contract check against $BASE_URL (run $RUN_ID)"
echo

# -------------------------------------------------------------- health ----

req GET /health
check "GET /health" 200 '.status == "ok" and .db == "ok"'

# ---------------------------------------------------------- geofences ----

req POST /geofences "{\"name\":\"Contract Restricted $RUN_ID\",\"description\":\"verify_contract.sh\",\"coordinates\":$SQUARE,\"category\":\"restricted_zone\"}"
check "POST /geofences -> 201 {id,name,status}" 201 \
  '(.id | test("^geo_")) and (.name | type == "string") and .status == "active"'
GEO_ID=$(jq -r '.id // empty' <<<"$BODY")

req GET /geofences
check "GET /geofences -> {geofences:[...]}" 200 \
  "(.geofences | type == \"array\") and (.geofences | map(select(.id == \"$GEO_ID\")) | length == 1)"
check "GET /geofences item fields & types" 200 \
  "(.geofences[] | select(.id == \"$GEO_ID\")) as \$g
   | (\$g.name | type == \"string\") and (\$g.description == \"verify_contract.sh\")
     and (\$g.category == \"restricted_zone\") and (\$g.created_at | test(\"^\\\\d{4}-\\\\d\\\\d-\\\\d\\\\dT.*Z$\"))
     and (\$g.coordinates | type == \"array\" and length == 5)
     and (\$g.coordinates | all(type == \"array\" and length == 2 and (.[0] | type == \"number\") and (.[1] | type == \"number\")))
     and (\$g.coordinates[0] == [$LAT0,$LNG0])"

req GET "/geofences?category=restricted_zone"
check "GET /geofences?category= filters" 200 \
  "(.geofences | length > 0) and (.geofences | all(.category == \"restricted_zone\")) and (.geofences | any(.id == \"$GEO_ID\"))"

req POST /geofences "{\"name\":\"Open\",\"coordinates\":[[1,1],[1,2],[2,2],[2,1]],\"category\":\"toll_zone\"}"
check_error "POST /geofences open polygon -> 400" 400 '.details.coordinates | test("closed")'
req POST /geofences "{\"name\":\"Bad\",\"coordinates\":$SQUARE,\"category\":\"parking\"}"
check_error "POST /geofences bad category -> 400" 400 '.details.category | type == "string"'
req POST /geofences "{\"name\":\"Lat\",\"coordinates\":[[91,0],[1,2],[2,2],[91,0]],\"category\":\"toll_zone\"}"
check_error "POST /geofences lat 91 -> 400" 400 '.details["coordinates[0]"] | test("latitude")'
req POST /geofences '{}'
check_error "POST /geofences missing fields -> 400" 400 '(.details.name | type == "string") and (.details.coordinates | type == "string") and (.details.category | type == "string")'
req GET "/geofences?category=parking"
check_error "GET /geofences?category=invalid -> 400" 400

# ----------------------------------------------------------- vehicles ----

VEH_NUM="CT-$RUN_ID"
VEH_NUM=${VEH_NUM:0:20}
req POST /vehicles "{\"vehicle_number\":\"$VEH_NUM\",\"driver_name\":\"Contract Driver\",\"vehicle_type\":\"truck\",\"phone\":\"+1234567890\"}"
check "POST /vehicles -> 201 {id,vehicle_number,status}" 201 \
  "(.id | test(\"^veh_\")) and .vehicle_number == \"$VEH_NUM\" and .status == \"active\""
VEH_ID=$(jq -r '.id // empty' <<<"$BODY")

req POST /vehicles "{\"vehicle_number\":\"$VEH_NUM\",\"driver_name\":\"Dup\",\"vehicle_type\":\"van\",\"phone\":\"+1987654321\"}"
check_error "POST /vehicles duplicate -> 409" 409
req POST /vehicles '{}'
check_error "POST /vehicles missing fields -> 400" 400 \
  '[.details.vehicle_number, .details.driver_name, .details.vehicle_type, .details.phone] | all(type == "string")'

req GET /vehicles
check "GET /vehicles item fields & types" 200 \
  "(.vehicles | type == \"array\") and ((.vehicles[] | select(.id == \"$VEH_ID\")) as \$v
   | \$v.vehicle_number == \"$VEH_NUM\" and \$v.driver_name == \"Contract Driver\" and \$v.vehicle_type == \"truck\"
     and \$v.phone == \"+1234567890\" and \$v.status == \"active\" and (\$v.created_at | test(\"Z$\")))"

# ------------------------------------------------------- alert rules ----

req POST /alerts/configure "{\"geofence_id\":\"$GEO_ID\",\"vehicle_id\":\"$VEH_ID\",\"event_type\":\"entry\"}"
check "POST /alerts/configure -> 201 {alert_id,...}" 201 \
  "(.alert_id | test(\"^alert_\")) and .geofence_id == \"$GEO_ID\" and .vehicle_id == \"$VEH_ID\" and .event_type == \"entry\" and .status == \"active\""
ALERT_ID=$(jq -r '.alert_id // empty' <<<"$BODY")

req POST /alerts/configure "{\"geofence_id\":\"$GEO_ID\",\"event_type\":\"exit\"}"
check "POST /alerts/configure all vehicles -> vehicle_id null" 201 '.vehicle_id == null and .event_type == "exit"'
ALL_ALERT_ID=$(jq -r '.alert_id // empty' <<<"$BODY")
req POST /alerts/configure "{\"geofence_id\":\"$GEO_ID\",\"event_type\":\"sometimes\"}"
check_error "POST /alerts/configure bad event_type -> 400" 400 '.details.event_type | type == "string"'
req POST /alerts/configure "{\"geofence_id\":\"geo_doesnotexist\",\"event_type\":\"entry\"}"
check_error "POST /alerts/configure unknown geofence -> 404" 404

req GET "/alerts?geofence_id=$GEO_ID"
check "GET /alerts?geofence_id= fields & types" 200 \
  "(.alerts | length == 2) and ((.alerts[] | select(.alert_id == \"$ALERT_ID\")) as \$a
   | \$a.geofence_id == \"$GEO_ID\" and (\$a.geofence_name | type == \"string\") and \$a.vehicle_id == \"$VEH_ID\"
     and \$a.vehicle_number == \"$VEH_NUM\" and \$a.event_type == \"entry\" and \$a.status == \"active\" and (\$a.created_at | test(\"Z$\")))"
req GET "/alerts?vehicle_id=$VEH_ID"
check "GET /alerts?vehicle_id= filters" 200 "(.alerts | length == 1) and .alerts[0].alert_id == \"$ALERT_ID\""

# -------------------------------------------- WebSocket listener (bg) ----

WS_CLIENT=""
if command -v websocat >/dev/null; then
  WS_CLIENT=websocat
  (sleep 25 | websocat -t "$WS_URL" >"$TMP/ws.out" 2>&1) &
  WS_PID=$!
elif command -v npx >/dev/null; then
  WS_CLIENT="npx wscat"
  (sleep 25 | npx -y wscat -c "$WS_URL" >"$TMP/ws.out" 2>&1) &
  WS_PID=$!
fi
if [[ -n $WS_CLIENT ]]; then
  for _ in $(seq 1 60); do grep -q '"connected"' "$TMP/ws.out" 2>/dev/null && break; sleep 0.25; done
fi

# ------------------------------------------- locations: out -> in -> out ----

post_location() { # lat lng seconds_ago
  req POST /vehicles/location "{\"vehicle_id\":\"$VEH_ID\",\"latitude\":$1,\"longitude\":$2,\"timestamp\":\"$(now_minus "$3")\"}"
}

post_location "$OUT_LAT" "$IN_LNG" 30
check "POST /vehicles/location (outside)" 200 \
  "(.vehicle_id == \"$VEH_ID\") and (.location_updated == true) and (.current_geofences | type == \"array\") and (.current_geofences | all(.geofence_id != \"$GEO_ID\"))"
post_location "$IN_LAT" "$IN_LNG" 20
check "POST /vehicles/location (inside) -> status inside" 200 \
  "(.current_geofences[] | select(.geofence_id == \"$GEO_ID\")) as \$g | (\$g.geofence_name | type == \"string\") and \$g.status == \"inside\""

req GET "/vehicles/location/$VEH_ID"
check "GET /vehicles/location/{id} fields & types" 200 \
  "(.vehicle_id == \"$VEH_ID\") and (.vehicle_number == \"$VEH_NUM\")
   and (.current_location.latitude == $IN_LAT) and (.current_location.longitude == $IN_LNG)
   and (.current_location.timestamp | test(\"Z$\"))
   and ((.current_geofences[] | select(.geofence_id == \"$GEO_ID\")) as \$g | (\$g.geofence_name | type == \"string\") and \$g.category == \"restricted_zone\")"

post_location "$OUT_LAT" "$IN_LNG" 10
check "POST /vehicles/location (outside again)" 200 "(.current_geofences | all(.geofence_id != \"$GEO_ID\"))"

post_location 91 0 5
check_error "POST /vehicles/location lat 91 -> 400" 400 '.details.latitude | type == "string"'
req POST /vehicles/location '{}'
check_error "POST /vehicles/location missing fields -> 400" 400 \
  '[.details.vehicle_id, .details.latitude, .details.longitude, .details.timestamp] | all(type == "string")'
req POST /vehicles/location "{\"vehicle_id\":\"veh_doesnotexist\",\"latitude\":1,\"longitude\":1,\"timestamp\":\"$(now_minus 1)\"}"
check_error "POST /vehicles/location unknown vehicle -> 404" 404
req GET /vehicles/location/veh_doesnotexist
check_error "GET /vehicles/location/{unknown} -> 404" 404

# ----------------------------------------------------------- history ----

req GET "/violations/history?vehicle_id=$VEH_ID&geofence_id=$GEO_ID"
check "GET /violations/history entry + exit recorded" 200 \
  '(.total_count == 2) and ([.violations[].event_type] == ["exit", "entry"])'
check "GET /violations/history item fields & types" 200 \
  "(.total_count | type == \"number\") and (.violations | all(
     (.id | test(\"^viol_\")) and .vehicle_id == \"$VEH_ID\" and .vehicle_number == \"$VEH_NUM\"
     and .geofence_id == \"$GEO_ID\" and (.geofence_name | type == \"string\")
     and (.latitude | type == \"number\") and (.longitude | type == \"number\") and (.timestamp | test(\"Z$\"))))"
req GET "/violations/history?vehicle_id=$VEH_ID&limit=1"
check "GET /violations/history?limit=1 (total ignores limit)" 200 '(.violations | length == 1) and .total_count == 2'
req GET "/violations/history?vehicle_id=$VEH_ID&start_date=$(now_minus 15)&end_date=$(now_minus 0)"
check "GET /violations/history start_date/end_date" 200 '(.total_count == 1) and .violations[0].event_type == "exit"'
req GET "/violations/history?start_date=yesterday"
check_error "GET /violations/history bad date -> 400" 400 '.details.start_date | type == "string"'

# --------------------------------------------- generic error contract ----

req GET /no/such/route
check_error "unknown route -> 404 JSON" 404
req DELETE /vehicles
check_error "wrong method -> 405 JSON" 405

# ---------------------------------------------------------- websocket ----

if [[ -z $WS_CLIENT ]]; then
  record FAIL "WS /ws/alerts entry alert received" "no websocat or npx available"
else
  # The rule is entry-only for this vehicle, and the all-vehicles rule is exit-only.
  for _ in $(seq 1 20); do
    grep -q "\"vehicle_id\":\"$VEH_ID\"" "$TMP/ws.out" 2>/dev/null && [[ $(grep -c "\"vehicle_id\":\"$VEH_ID\"" "$TMP/ws.out") -ge 2 ]] && break
    sleep 0.25
  done
  ALERTS=$(grep -o '{.*}' "$TMP/ws.out" | jq -c "select(.event_id? and .vehicle.vehicle_id == \"$VEH_ID\")" 2>/dev/null)
  if jq -se 'map(select(.event_type == "entry")) | length == 1' <<<"$ALERTS" >/dev/null 2>&1 &&
    jq -se '(map(select(.event_type == "entry"))[0]) as $a
      | ($a | keys) == ["event_id","event_type","geofence","location","timestamp","vehicle"]
        and ($a.vehicle | keys) == ["driver_name","vehicle_id","vehicle_number"]
        and ($a.geofence | keys) == ["category","geofence_id","geofence_name"]
        and ($a.location | keys) == ["latitude","longitude"]
        and ($a.event_id | test("^evt_")) and ($a.geofence.category == "restricted_zone")' <<<"$ALERTS" >/dev/null 2>&1; then
    record PASS "WS /ws/alerts entry alert received ($WS_CLIENT)" "exact spec shape"
  else
    record FAIL "WS /ws/alerts entry alert received ($WS_CLIENT)" "got: $(head -c 200 <<<"$ALERTS$(tail -c 200 "$TMP/ws.out")")"
  fi
  if jq -se 'map(select(.event_type == "exit")) | length == 1' <<<"$ALERTS" >/dev/null 2>&1; then
    record PASS "WS exit alert via all-vehicles rule (vehicle_id null)" "1 exit alert"
  else
    record FAIL "WS exit alert via all-vehicles rule (vehicle_id null)" "exit alerts: $(jq -sc 'map(select(.event_type=="exit"))|length' <<<"$ALERTS")"
  fi
fi

# ---------------------------------------------------- cleanup (extra) ----
# Remove the rules this run created (PATCH/DELETE are additive endpoints).
for id in "$ALERT_ID" "$ALL_ALERT_ID"; do [[ -n $id ]] && curl -s -o /dev/null -X DELETE "$BASE_URL/alerts/$id"; done

# ------------------------------------------------------------ report ----

printf '\n%-6s  %-58s  %s\n' RESULT CHECK DETAIL
printf '%-6s  %-58s  %s\n' ------ ---------------------------------------------------------- ------
for row in "${RESULTS[@]}"; do
  IFS='|' read -r status name detail <<<"$row"
  if [[ -t 1 ]]; then
    [[ $status == PASS ]] && status=$'\e[32mPASS\e[0m  ' || status=$'\e[31mFAIL\e[0m  '
    printf '%s  %-58s  %s\n' "$status" "$name" "$detail"
  else
    printf '%-6s  %-58s  %s\n' "$status" "$name" "$detail"
  fi
done
echo
echo "$PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]]
