#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
for tool in go restic autorestic asciinema agg ffmpeg expect node; do
    command -v "$tool" >/dev/null || { printf 'Missing capture dependency: %s\n' "$tool" >&2; exit 1; }
done

work=$(mktemp -d /tmp/restic-studio-capture.XXXXXX)
cleanup() {
    rm -f "$work/restic-studio" "$work/password" "$work/autorestic.yml" "$work/frames.txt"
    # Only this newly created demo directory is removed.
    rm -r "$work"
}
trap cleanup EXIT
mkdir -p docs/images docs/recordings "$work/documents"
printf 'A generated document for the Restic Studio demonstration.\n' > "$work/documents/notes.txt"
printf 'Another harmless generated file.\n' > "$work/documents/tasks.txt"
printf 'capture-only-demo-password\n' > "$work/password"
chmod 600 "$work/password"
export RESTIC_PASSWORD_FILE="$work/password"
export RESTIC_REPOSITORY="$work/repository"
restic --no-cache init >/dev/null
restic --no-cache backup "$work/documents" --host demo-laptop --tag documentation >/dev/null
cat > "$work/autorestic.yml" <<EOF
version: 2
locations:
  documents:
    from: "$work/documents"
    to: demo-local
backends:
  demo-local:
    type: local
    path: "$work/repository"
    env:
      RESTIC_PASSWORD_FILE: "$work/password"
EOF
chmod 600 "$work/autorestic.yml"
go build -o "$work/restic-studio" ./cmd/restic-studio

export CAPTURE_WORK="$work"
export TMPDIR="$work"
export TERM=xterm-256color
export COLORTERM=truecolor
asciinema rec --headless --overwrite --output-format asciicast-v2 \
    --window-size 140x36 --return --title "Restic Studio: tests" \
    --command 'bash -c "printf \"\$ go test -race -count=1 ./...\n\"; go test -race -count=1 ./..."' \
    docs/recordings/tests.cast
asciinema rec --headless --overwrite --output-format asciicast-v2 \
    --window-size 140x36 --return --title "Restic Studio: isolated repository walkthrough" \
    --command 'expect docs/capture.exp' docs/recordings/walkthrough.cast

if grep -q 'capture-only-demo-password' docs/recordings/*.cast; then
    printf 'A demo password appeared in a recording; do not publish it.\n' >&2
    exit 1
fi

agg --quiet --font-size 16 --idle-time-limit 60 --last-frame-duration 3 \
    docs/recordings/walkthrough.cast docs/images/walkthrough.gif
node <<'NODE' > "$work/frames.txt"
const fs = require("node:fs");
const events = fs.readFileSync("docs/recordings/walkthrough.cast", "utf8")
    .trim().split("\n").slice(1).map(JSON.parse);
events.forEach((event, index) => {
    if (event[1] !== "o") return;
    const marker = event[2].match(/\x1b\]777;capture;([a-z-]+)\x07/);
    if (marker) console.log(`event:${index} ${marker[1]}`);
});
NODE
while read -r position name; do
    agg --quiet --font-size 16 --idle-time-limit 60 --select "$position" \
        docs/recordings/walkthrough.cast "$work/frame.gif"
    ffmpeg -hide_banner -loglevel error -y -i "$work/frame.gif" \
        -frames:v 1 -update 1 "docs/images/$name.png"
done < "$work/frames.txt"

agg --quiet --font-size 16 --idle-time-limit 60 --select '100%' \
    docs/recordings/tests.cast "$work/tests.gif"
ffmpeg -hide_banner -loglevel error -y -i "$work/tests.gif" \
    -frames:v 1 -update 1 docs/images/tests.png
for image in dashboard locations backup-confirmation repository-tools repository-check snapshots restore-confirmation tests; do
    test -s "docs/images/$image.png"
done
printf 'Saved verified recordings and screenshots in docs/recordings and docs/images.\n'
