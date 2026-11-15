#!/usr/bin/env bash
# Weekly upstream upgrade probe of the LEO patch queue (GOWA PLAN F6-T2).
#
# 1. fetch the upstream tags; NEW = the highest semver tag (vX.Y.Z) above the base tag
#    (leo/BASE_TAG, or BASE_OVERRIDE for a test run). None -> status=none, exit 0;
# 2. branch upgrade/$NEW from $NEW and cherry-pick -x, one by one and in order, the LEO
#    queue = the non-merge commits of $MAIN_REF not reachable from the REAL base tag
#    (leo/BASE_TAG on $MAIN_REF; the override only moves NEW, never the queue);
#    - first conflict: abort that pick, keep the picks done so far, record the sha and
#      the conflicted files -> status=conflict;
#    - no conflict: leo/BASE_TAG := $NEW -> status=clean;
# 3. push upgrade/$NEW. Idempotent: when the remote branch already exists it is NEVER
#    overwritten (existed=true): the queue is replayed locally only, to recompute the
#    verdict. The verdict itself goes to an issue (leo/probe/issue-verdict.sh): GitHub
#    Actions may not open PRs in the LEO-ENT organisation.
#
# Env: UPSTREAM_URL, MAIN_REF (default origin/main), BASE_OVERRIDE (optional),
#      PUSH=1 to push (CI), else local only (simulation),
#      GITHUB_OUTPUT / GITHUB_STEP_SUMMARY (optional).
# Outputs: status, new_tag, base_tag, real_base_tag, branch, existed, main_sha, queue_len,
#      picked, empty_picks, conflict_sha, conflict_files, queue_md (multi-line Markdown).
set -euo pipefail

UPSTREAM_URL="${UPSTREAM_URL:-https://github.com/aldinokemal/go-whatsapp-web-multidevice.git}"
MAIN_REF="${MAIN_REF:-origin/main}"
BASE_OVERRIDE="${BASE_OVERRIDE:-}"
PUSH="${PUSH:-0}"
OUT="${GITHUB_OUTPUT:-/dev/stdout}"
WORK="${PROBE_WORKDIR:-$(mktemp -d)}"
SEMVER='^v[0-9]+\.[0-9]+\.[0-9]+$'

out() { printf '%s=%s\n' "$1" "$2" >>"$OUT"; }
summary() {
  if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then printf '%s\n' "$*" >>"$GITHUB_STEP_SUMMARY"; fi
}
die() { echo "::error title=upgrade-probe::$*" >&2; exit 1; }

git fetch --quiet --force --no-tags "$UPSTREAM_URL" '+refs/tags/*:refs/tags/*' || die "fetch of upstream tags failed ($UPSTREAM_URL)"

REAL_BASE="$(git show "$MAIN_REF:leo/BASE_TAG" | tr -d '[:space:]')"
[[ "$REAL_BASE" =~ $SEMVER ]] || die "leo/BASE_TAG on $MAIN_REF is not a vX.Y.Z tag: '$REAL_BASE'"
BASE="${BASE_OVERRIDE:-$REAL_BASE}"
[[ "$BASE" =~ $SEMVER ]] || die "base tag override is not a vX.Y.Z tag: '$BASE'"
git rev-parse --quiet --verify "refs/tags/$REAL_BASE^{commit}" >/dev/null || die "real base tag $REAL_BASE not found upstream"
git rev-parse --quiet --verify "refs/tags/$BASE^{commit}" >/dev/null || die "base tag $BASE not found upstream"
out base_tag "$BASE"
out real_base_tag "$REAL_BASE"

NEW="$(git tag -l 'v*' | grep -E "$SEMVER" | { cat; echo "$BASE"; } | sort -uV | tail -1)"
if [[ "$NEW" == "$BASE" ]]; then
  echo "no upstream tag above $BASE"
  out status none
  summary "No upstream tag above \`$BASE\`: nothing to probe."
  exit 0
fi
BRANCH="upgrade/$NEW"
out new_tag "$NEW"
out branch "$BRANCH"
echo "base=$BASE real_base=$REAL_BASE new=$NEW branch=$BRANCH"

existed=false
if [[ "$PUSH" == "1" ]] && git ls-remote --exit-code --heads origin "$BRANCH" >/dev/null 2>&1; then
  existed=true
  echo "branch $BRANCH already exists: not overwritten, verdict recomputed locally"
fi
out existed "$existed"
out main_sha "$(git rev-parse "$MAIN_REF^{commit}")"

mapfile -t QUEUE < <(git rev-list --reverse --no-merges "$MAIN_REF" --not "refs/tags/$REAL_BASE")
[[ ${#QUEUE[@]} -gt 0 ]] || die "empty LEO queue: $MAIN_REF has no commit above $REAL_BASE"
echo "queue: ${#QUEUE[@]} commits ($REAL_BASE..$MAIN_REF)"
out queue_len "${#QUEUE[@]}"

git checkout --quiet -B "$BRANCH" "refs/tags/$NEW^{commit}"
picked=0
empty=()
status=clean
conflict_sha=""
conflict_files=""
for sha in "${QUEUE[@]}"; do
  before="$(git rev-parse HEAD)"
  if git cherry-pick -x --allow-empty --keep-redundant-commits "$sha" >"$WORK/pick.log" 2>&1; then
    picked=$((picked + 1))
    if [[ "$(git rev-parse "HEAD^{tree}")" == "$(git rev-parse "$before^{tree}")" ]] \
      && [[ -n "$(git diff-tree --no-commit-id --name-only -r "$sha")" ]]; then
      empty+=("${sha:0:7}")  # its change is already upstream: a DROP candidate
    fi
    continue
  fi
  conflict_files="$(git diff --name-only --diff-filter=U | paste -sd, -)"
  if [[ -z "$conflict_files" ]]; then
    cat "$WORK/pick.log" >&2
    git cherry-pick --abort >/dev/null 2>&1 || true
    die "cherry-pick of $sha failed without a merge conflict"
  fi
  git cherry-pick --abort
  status=conflict
  conflict_sha="$sha"
  break
done

out picked "$picked"
out empty_picks "$(IFS=,; echo "${empty[*]:-}")"
out conflict_sha "$conflict_sha"
out conflict_files "$conflict_files"

if [[ "$status" == clean ]]; then
  printf '%s\n' "$NEW" >leo/BASE_TAG
  if ! git diff --quiet -- leo/BASE_TAG; then
    git add leo/BASE_TAG
    git commit --quiet -m "chore(leo): rebase the patch queue on $NEW"
  fi
fi
out status "$status"
echo "status=$status picked=$picked/${#QUEUE[@]} empty=${empty[*]:-} conflict=${conflict_sha:-} files=${conflict_files:-}"

md="$WORK/queue.md"
{
  echo "- LEO queue: ${#QUEUE[@]} commits of \`$MAIN_REF\` (\`$(git rev-parse --short "$MAIN_REF^{commit}")\`) above \`$REAL_BASE\`, picked $picked with \`cherry-pick -x\`"
  if [[ ${#empty[@]} -gt 0 ]]; then
    echo "- picks left empty on \`$NEW\` (DROP candidates for \`leo/PATCHES.md\`): ${empty[*]}"
  fi
  if [[ "$status" == conflict ]]; then
    echo "- **CONFLICT** on the pick of \`$conflict_sha\` (\`$(git log -1 --format=%s "$conflict_sha")\`), files:"
    # shellcheck disable=SC2016  # literal Markdown backticks
    tr ',' '
' <<<"$conflict_files" | sed 's/^/  - `/; s/$/`/'
    echo "- the branch holds the $picked picks that applied; not picked yet:"
    started=0
    for sha in "${QUEUE[@]}"; do
      [[ "$sha" == "$conflict_sha" ]] && started=1
      [[ $started == 1 ]] && echo "  - \`${sha:0:7}\` $(git log -1 --format=%s "$sha")"
    done
  else
    echo "- clean: \`leo/BASE_TAG\` = \`$NEW\` on the branch"
  fi
} >"$md"
cat "$md"
delim="QUEUE_MD_$(od -An -N8 -tx1 /dev/urandom | tr -d ' 
')"
{ echo "queue_md<<$delim"; cat "$md"; echo "$delim"; } >>"$OUT"

if [[ "$PUSH" != "1" ]]; then
  echo "PUSH!=1: local simulation, nothing pushed"
  exit 0
fi
if [[ "$existed" == true ]]; then
  summary "\`$BRANCH\` already existed: not overwritten; verdict recomputed ($status). Delete the branch to re-push."
  exit 0
fi

if ! git push --quiet origin "refs/heads/$BRANCH:refs/heads/$BRANCH" 2>"$WORK/push.err"; then
  cat "$WORK/push.err" >&2
  if grep -q "workflows\` permission\|workflow.*permission" "$WORK/push.err"; then
    die "push of $BRANCH refused: GITHUB_TOKEN cannot create or update workflow files; a token with the workflows permission is needed"
  fi
  die "push of $BRANCH failed"
fi
summary "Pushed \`$BRANCH\` ($status)."
