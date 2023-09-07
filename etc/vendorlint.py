#!/usr/bin/env python3
"""vendorlint -- walk go vendor directory and lint projects"""

import argparse, os, logging, subprocess, json, collections, multiprocessing
from typing import List, Tuple

logger = logging.getLogger(__name__)

VCS_PREFIXES = ['github.com', 'gitlab.com']
INCLUDE_ISSUES = ['SA1027:']

def test_path(path: str) -> bool:
    "return True if we should lint this path"
    head, *tail = path.split(os.path.sep)
    tail_len = 2 if head in VCS_PREFIXES else 1
    return len(tail) == tail_len

def ensure_suffix(base: str, suffix: str) -> str:
    "add suffix to base if necessary. not smart about multi-character suffixes"
    return base if base.endswith(suffix) else base + suffix

def lint_subdir(args, mod: str, path: str):
    "run lint on a directory"
    logger.info('linting %s', path)
    if args.dry_run:
        return (mod, None)
    # todo: govet by itself may run atomicalign -- is that faster / smaller?
    # note: --no-config flag is so vendor's own lint config doesn't break this
    proc = subprocess.run(f"{args.linter} run -v --tests=false --disable-all --enable staticcheck --out-format json --no-config ./...", cwd=path, shell=True, check=False, capture_output=True)
    if proc.returncode != 0:
        logger.error('bad result %d mod %s OUT %s... ERR %s...', proc.returncode, mod, proc.stdout[:40], proc.stderr[:40])
    return (mod, (json.loads(proc.stdout) if proc.stdout else None))

def walk(args) -> dict:
    "walk vendor tree, produce json results"
    if not os.path.exists(args.root):
        raise EnvironmentError(f'args.root ${args.root} does not exist')
    prefix = ensure_suffix(args.root, '/')
    targets: List[Tuple[str, str]] = []
    for path, _, _ in os.walk(args.root):
        mod = path.removeprefix(prefix)
        if test_path(mod):
            targets.append((mod, path))
    with multiprocessing.Pool(args.parallel) as pool:
        results = dict(pool.starmap(lint_subdir, [(args, *tup) for tup in targets]))
    if args.out:
        logger.info("writing result to file %s", args.out)
        with open(args.out, 'w') as fout:
            json.dump(results, fout)
    return results

class FailingModules(Exception):
    pass

def analyze(args):
    "filter json results to actually relevant, throw error if result looks bad"
    with open(args.out) as f_in:
        results = json.load(f_in)
    ignored = collections.Counter()
    included = []
    for mod, result in results.items():
        if result is None:
            logger.warning('empty mod %s', mod)
            continue
        for issue in result['Issues']:
            token = issue['Text'].split()[0]
            if token not in INCLUDE_ISSUES:
                ignored[token] += 1
                continue
            logger.warning("included: %s %s %s", mod, issue['FromLinter'], issue['Text'])
            included.append((mod, issue))
    logger.info('ignored %s', ignored)
    if included:
        raise FailingModules(f"{len(included)} {collections.Counter(mod for mod, _ in included)}")

def main():
    p = argparse.ArgumentParser()
    p.add_argument('command', choices=('walk', 'analyze', 'all'))
    p.add_argument('--root', default='vendor', help="where to start the walk")
    p.add_argument('--dry-run', action='store_true', help="don't lint, just log")
    p.add_argument('--out', default='vendorlint.json', help="store json result to file")
    p.add_argument('--linter', default="golangci-lint", help="path to linter")
    p.add_argument('--parallel', '-p', type=int, help="parallelism. by default use os.cpu_count")
    args = p.parse_args()
    logging.basicConfig(level=logging.INFO)

    if args.command in ('walk', 'all'):
        walk(args)
    if args.command in ('analyze', 'all'):
        analyze(args)

if __name__ == '__main__':
    main()
