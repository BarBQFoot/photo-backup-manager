#!/bin/bash
# Build on the Mac itself. The produced .app prepares Python/AI in its own UI.
set -euo pipefail
export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/local/go/bin:/opt/homebrew/opt/node@24/bin:/usr/local/opt/node@24/bin:$HOME/go/bin:$PATH"
cd "$(dirname "$0")"
project_root="$PWD"
package_stage=""

finish() {
    status=$?
    if [ -n "$package_stage" ] && [ -d "$package_stage" ]; then
        case "$package_stage" in
            "$project_root"/build/bin/mac-release/package.*) /bin/rm -rf -- "$package_stage" ;;
            *) echo "Skipped cleanup: unexpected staging path." ;;
        esac
    fi
    if [ "$status" -ne 0 ]; then
        printf '\nBuild failed. Read the error above, then try again.\n'
    fi
    printf '\nPress Enter to close... '
    read -r _ || true
}
trap finish EXIT

if [ "$(uname -s)" != "Darwin" ]; then
    echo "Run this file on the Mac. A native macOS build needs Apple's SDK."
    exit 1
fi
if [ ! -f go.mod ] || [ ! -f wails.json ]; then
    echo "Keep this file in the latest photo-backup-manager project folder."
    exit 1
fi
if ! xcode-select -p >/dev/null 2>&1; then
    echo "Installing Xcode Command Line Tools. Finish Apple's installer, then run this file again."
    xcode-select --install || true
    exit 1
fi

missing_tools=()
if ! command -v go >/dev/null 2>&1; then missing_tools+=(go); fi
if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then missing_tools+=(node@24); fi
if [ "${#missing_tools[@]}" -gt 0 ]; then
    if ! command -v brew >/dev/null 2>&1; then
        echo "Install Go and Node.js 24 first, then run this file again."
        echo "Go: https://go.dev/dl/"
        echo "Node: https://nodejs.org/en/download"
        exit 1
    fi
    HOMEBREW_NO_ASK=1 brew install "${missing_tools[@]}"
    if command -v brew >/dev/null 2>&1; then
        export PATH="$(brew --prefix)/opt/node@24/bin:$PATH"
    fi
fi
export PATH="$(go env GOPATH)/bin:$PATH"
if ! node -e 'process.exit(Number(process.versions.node.split(".")[0]) >= 22 ? 0 : 1)'; then
    echo "Use Node.js 24 LTS to build this project."
    exit 1
fi
if ! command -v wails >/dev/null 2>&1 || [[ "$(wails version 2>/dev/null)" != *v2.13.0* ]]; then
    echo "Installing Wails v2.13.0 for this build..."
    go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
fi

case "$(uname -m)" in
    arm64) target="arm64" ;;
    x86_64) target="amd64" ;;
    *) echo "Unsupported Mac architecture."; exit 1 ;;
esac
echo "Building macOS $target. No Python or model installation is needed to build."
wails build -platform "darwin/$target" -o photo-backup-manager
bundle="$project_root/build/bin/photo-backup-manager.app"
if [ ! -d "$bundle" ]; then
    echo "Build did not produce the expected .app bundle."
    exit 1
fi

release="$project_root/build/bin/mac-release"
mkdir -p "$release"
package_stage="$(mktemp -d "$release/package.XXXXXX")"
ditto "$bundle" "$package_stage/photo-backup-manager.app"
printf 'GEMINI_API_KEY=\n' > "$package_stage/.env"
cat > "$package_stage/อ่านก่อนเปิดโปรแกรม.txt" <<'README'
Photo Backup Manager สำหรับ Mac

ดับเบิลคลิก photo-backup-manager.app แล้วรอหน้าจอเตรียม AI
ส่วนที่มีอยู่แล้วจะถูกข้าม ดาวน์โหลดเฉพาะส่วนที่ขาด ใช้ MPS เมื่อพร้อม มิฉะนั้นใช้ CPU
ครั้งแรกต้องมีอินเทอร์เน็ตและพื้นที่หลาย GB ไม่ต้องติดตั้ง Python/Go/Node/Wails บนเครื่องผู้รับ

ถ้าจะใช้แปลไทย เปิด .env ด้วย TextEdit แบบ plain text ใส่ Gemini API key หลัง = แล้วเปิดแอปใหม่
วาง .env ข้างไฟล์ .app ไม่ต้องเปิดเข้าไปแก้ใน Contents ของแอป
ZIP นี้มี .env เปล่า ไม่รวมคีย์จากเครื่องที่ build
ถ้าย้าย .app ไป Applications ให้ย้าย .env ไปข้าง .app ด้วย หรือใช้ ~/Library/Application Support/photo-backup-manager/.env

ฐานข้อมูลแยกจาก dev อยู่ที่ ~/Library/Application Support/photo-backup-manager/photo_backup.db
Python และแคชแอปอยู่ที่ ~/Library/Caches/photo-backup-manager/ai
โมเดลใช้ Hugging Face cache ของบัญชีผู้ใช้ตามเดิม
แอปที่ build นี้ยังไม่ได้เซ็นด้วย Apple Developer ID / notarize
README
archive="$release/photo-backup-manager-macos-$target.zip"
ditto -c -k --sequesterRsrc "$package_stage" "$archive"
echo
echo "App: $bundle"
echo "Share this ZIP (no API key included): $archive"
echo "Open the .app to see the automatic AI setup screen."
open -R "$archive"
