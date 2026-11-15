#!/usr/bin/env bash
# Verdict of the upstream upgrade probe as an ISSUE (GOWA PLAN F6-T2).
#
# GitHub Actions may not open pull requests in the LEO-ENT organisation, so the verdict
# lives in one issue per upstream tag, titled "upgrade/<tag>: GREEN|RED (...)" and
# labelled $LABEL. Idempotent: an OPEN issue whose title starts with "upgrade/<tag>:" is
# updated (title + body) and gets a comment with the new verdict; otherwise one is created.
# A missing label is created; when that is not possible the issue goes without it.
#
# Env (all from the workflow): REPO, SERVER_URL, RUN_URL, NEW, BASE, BRANCH, MAIN_SHA,
#   STATUS (clean|conflict|''), EXISTED, QUEUE_MD, PROBE_RESULT, CI_RESULT,
#   OASDIFF_RESULT, OASDIFF_MD, LABEL (default upstream-upgrade), GH (default gh).
set -euo pipefail

LABEL="${LABEL:-upstream-upgrade}"
GH="${GH:-gh}"
: "${REPO:?}" "${NEW:?}" "${RUN_URL:?}"
SERVER_URL="${SERVER_URL:-https://github.com}"
BRANCH="${BRANCH:-upgrade/$NEW}"
WORK="${PROBE_WORKDIR:-$(mktemp -d)}"
prefix="$BRANCH:"

reasons=()
[[ "${PROBE_RESULT:-}" == success ]] || reasons+=("probe ${PROBE_RESULT:-unknown}")
[[ "${STATUS:-}" == conflict ]] && reasons+=("cherry-pick conflict")
if [[ "${STATUS:-}" == clean && "${CI_RESULT:-}" != success ]]; then
  reasons+=("fork CI ${CI_RESULT:-not run}")
fi
[[ "${OASDIFF_RESULT:-}" == success ]] || reasons+=("oasdiff ${OASDIFF_RESULT:-not run}")
if [[ ${#reasons[@]} -eq 0 ]]; then
  verdict=GREEN
  title="$prefix GREEN"
else
  verdict=RED
  joined="$(IFS=,; echo "${reasons[*]}")"
  title="$prefix RED (${joined//,/, })"
fi

body="$WORK/issue-body.md"
{
  echo "Automatic upstream upgrade probe (GOWA PLAN F6-T2). Verdict: **$verdict**."
  echo
  echo "| | |"
  echo "|---|---|"
  echo "| upstream tag | \`$NEW\` (base \`${BASE:-?}\`) |"
  echo "| fork main | \`${MAIN_SHA:-?}\` |"
  echo "| cherry-pick | \`${STATUS:-not run}\` |"
  echo "| fork CI on the branch (workflow_call) | \`${CI_RESULT:-not run}\` |"
  echo "| oasdiff on the endpoints orch uses | \`${OASDIFF_RESULT:-not run}\` |"
  echo "| run | $RUN_URL |"
  echo
  echo "Branch: $SERVER_URL/$REPO/tree/$BRANCH"
  echo "Open the PR in one click: $SERVER_URL/$REPO/compare/main...$BRANCH?expand=1"
  if [[ "${EXISTED:-false}" == true ]]; then
    echo
    echo "> The branch already existed and was **not** overwritten: the cherry-pick verdict above"
    echo "> is recomputed on the current fork main; CI ran on the existing branch. Delete the"
    echo "> branch to let the probe push a fresh one."
  fi
  echo
  echo "## Cherry-pick of the LEO queue"
  echo
  printf '%s\n' "${QUEUE_MD:-_no queue report (the probe did not get that far)_}"
  echo
  echo "## oasdiff"
  echo
  printf '%s\n' "${OASDIFF_MD:-_not run_}"
  echo
  echo "Going to prod stays the manual GoWA train (leo-infra \`deploy/train/gowa/README.md\`)."
} >"$body"

label_args=()
if "$GH" label create "$LABEL" --repo "$REPO" --color 1D76DB \
  --description "Upstream GoWA upgrade probe verdicts" >/dev/null 2>"$WORK/label.err" \
  || "$GH" label list --repo "$REPO" --limit 200 --json name --jq '.[].name' 2>/dev/null \
    | grep -qxF "$LABEL"; then
  label_args=(--label "$LABEL")
else
  echo "::warning title=issue-verdict::label $LABEL unavailable, issue without label"
fi

number="$("$GH" issue list --repo "$REPO" --state open --limit 200 --json number,title \
  --jq ".[] | select(.title | startswith(\"$prefix\")) | .number" | head -1)"

if [[ -n "$number" ]]; then
  edit_label=()
  [[ ${#label_args[@]} -gt 0 ]] && edit_label=(--add-label "$LABEL")
  "$GH" issue edit "$number" --repo "$REPO" --title "$title" --body-file "$body" "${edit_label[@]}"
  "$GH" issue comment "$number" --repo "$REPO" --body "New probe verdict: **$verdict** ($RUN_URL)"
  echo "updated issue #$number: $title"
else
  "$GH" issue create --repo "$REPO" --title "$title" --body-file "$body" "${label_args[@]}"
  echo "created issue: $title"
fi
if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  echo "Probe verdict **$verdict**: $title" >>"$GITHUB_STEP_SUMMARY"
fi
