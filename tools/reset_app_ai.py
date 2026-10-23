"""Reset application AI descriptions and caption cache, preserving photos and models.

Defaults to inspection only. --execute backs up the SQLite DB and caption cache first.
"""
import argparse
from datetime import datetime
import json
from pathlib import Path
import shutil
import sqlite3
import zipfile

try:
    from .translate_captions import run_lock
except ImportError:
    from translate_captions import run_lock

ROOT = Path(__file__).resolve().parents[1]


def reset(db_path, cache, execute=False):
    db_path, cache = db_path.resolve(strict=True), cache.resolve(strict=True)
    # This utility intentionally targets only the known application caption cache.
    if cache.name != 'captions' or cache.parent.name != 'ai' or cache.parent.parent.name != 'photo-backup-manager':
        raise ValueError('Refusing a path outside the application AI caption cache')
    if not cache.is_dir():
        raise ValueError('Caption cache is not a directory')
    with run_lock(cache):
        paths = list(cache.rglob('*'))
        for path in paths:
            if path.is_symlink() or path.is_junction() or not path.resolve().is_relative_to(cache):
                raise ValueError('Refusing symlink/junction or path outside caption cache')
        db = sqlite3.connect(db_path.as_uri() + '?mode=rw', uri=True, timeout=5)
        try:
            if db.execute("SELECT count(*) FROM file_records WHERE ai_status='analyzing'").fetchone()[0]:
                raise ValueError('Analysis is still marked as running; stop and inspect before resetting')
            if db.execute("SELECT count(*) FROM backup_jobs WHERE status='running'").fetchone()[0]:
                raise ValueError('A file move is running; wait before resetting')
            count = db.execute('SELECT count(*) FROM file_records').fetchone()[0]
            report = {'records': count, 'caption_cache': len(list(cache.glob('caption_*.json'))),
                      'thai_cache': len(list(cache.glob('thai_*.json'))), 'executed': execute}
            if not execute:
                return report
            backup = ROOT / 'results' / ('ai_reset_' + datetime.now().strftime('%Y%m%d_%H%M%S_%f'))
            backup.mkdir(parents=True, exist_ok=False)
            with sqlite3.connect(backup / 'before_reset.db') as destination:
                db.backup(destination)
            db.execute('BEGIN IMMEDIATE')
            if db.execute("SELECT count(*) FROM file_records WHERE ai_status='analyzing'").fetchone()[0]:
                raise ValueError('An analysis started during preparation; reset cancelled')
            with zipfile.ZipFile(backup / 'caption_cache.zip', 'w', compression=zipfile.ZIP_DEFLATED) as archive:
                for path in paths:
                    if path.is_file() and path.name != '.translation.lock':
                        archive.write(path, path.relative_to(cache))
            # All targets were resolved and checked under the named cache above.
            # Keep the lock file open and in place until the complete operation finishes.
            for child in cache.iterdir():
                if child.name == '.translation.lock':
                    continue
                if not child.resolve().is_relative_to(cache):
                    raise ValueError('Cache path escaped its root')
                if child.is_dir():
                    shutil.rmtree(child)
                else:
                    child.unlink()
            db.execute("UPDATE file_records SET description='', ai_status='unanalyzed'")
            db.commit()
            remaining = db.execute("SELECT count(*) FROM file_records WHERE coalesce(description,'')!='' OR ai_status!='unanalyzed'").fetchone()[0]
            report.update(backup=str(backup), records_with_ai_remaining=remaining,
                          caption_cache_remaining=len(list(cache.glob('caption_*.json'))),
                          thai_cache_remaining=len(list(cache.glob('thai_*.json'))))
            (backup / 'reset_report.json').write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding='utf-8')
            return report
        finally:
            db.close()


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--db', required=True, type=Path)
    parser.add_argument('--cache', required=True, type=Path)
    parser.add_argument('--execute', action='store_true')
    args = parser.parse_args()
    if args.cache.is_symlink() or args.cache.is_junction():
        raise SystemExit('Refusing a linked cache root')
    print(json.dumps(reset(args.db, args.cache, args.execute), ensure_ascii=False, indent=2))
