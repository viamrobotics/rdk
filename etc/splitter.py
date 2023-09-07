#!/usr/bin/env python3
"consume output of `go list -json` and run a partition of tests"

import argparse, json, sys, logging, subprocess
import os
from typing import List, Tuple, Iterable, TypeVar, Callable

# SizePair is (package, num_tests)
SizePair = Tuple[str, int]
SizeList = List[SizePair]
logger = logging.getLogger(__name__)

# Dict of {package1: package2}
# The package paths in here are what `go list` outputs, i.e. './relative' style.
# This causes package1 to be run directly after package2.
PLACEMENT_RULES = {
    # pointcloud relies on artifacts created in videosource
    "./pointcloud": "./components/camera/videosource",
}

def file_or_stdin(path: str):
    "return file that can be used as nullcontext"
    if path == '-':
        return sys.stdin
    else:
        return open(path, 'rb')

def iter_scan_once(raw: str):
    """
    Yield top-level json objects from input string.
    This is to deal with inputs that are formatted as {}\n{} rather than [{},{}]
    """
    jd = json.JSONDecoder()
    offset = 0
    while 1:
        try:
            item, offset = jd.scan_once(raw, offset)
            yield item
            offset += 1 # to deal with \n
        except StopIteration:
            break

def split_bins(sizes: SizeList, bin_size: int) -> List[List[str]]:
    "given (name, size) in sizes, return [[name], [name]] where the sublists are approximately bin_size"
    ret = []
    accum = 0
    i = 0
    for j, (_, size) in enumerate(sizes):
        accum += size
        if accum >= bin_size:
            ret.append([x for x, _ in sizes[i:j + 1]])
            logger.debug('turnover: accum %d items %d slice [%d:%d]', accum, len(ret[-1]), i, j + 1)
            i = j + 1
            accum = 0
    if i < len(sizes):
        ret.append([x for x, _ in sizes[i:]])
    assert sum(len(x) for x in ret) == len(sizes), "split_bins lost values"
    assert len(set(y for x in ret for y in x)) == len(set(sizes)), "split_bins has different number of unique values than input"
    return ret

T = TypeVar('T')
def partition(sequence: Iterable[T], pred: Callable[[T], bool]) -> Tuple[List[T], List[T]]:
    "same as more_itertools.partition"
    false_items = []
    true_items = []
    for item in sequence:
        if pred(item):
            true_items.append(item)
        else:
            false_items.append(item)
    return false_items, true_items

U = TypeVar('U')
def keyed_index(sequence: Iterable[T], value: U, key: Callable[[T], U]) -> int:
    "helper to get index of iterable with key transformation"
    for i, x in enumerate(sequence):
        if key(x) == value:
            return i
    raise ValueError('not found in sequence')

def apply_placement(sizes: SizeList) -> SizeList:
    "apply placement rules, i.e. 'must come directly after' relationships (limited topo-sort)"
    unmoved, moved = partition(sizes, lambda item: item[0] in PLACEMENT_RULES)
    for item in moved:
        target = PLACEMENT_RULES[item[0]]
        index = keyed_index(unmoved, target, lambda x: x[0]) + 1
        unmoved.insert(index, item)
        logger.info('placement: inserted %s after %s at %d', item[0], target, index)
    return unmoved

def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('index', type=int, help="")
    p.add_argument('-p', '--path', default='-', help="path of json output. default is '-', i.e. stdin")
    p.add_argument('-n', '--nbins', type=int, default=5, help="number of bins to split up tests")
    p.add_argument('--dry-run', action='store_true', help="print output but don't run anything")
    p.add_argument('-d', '--debug', action='store_true', help="log level debug")
    p.add_argument('-c', '--command', default="go test", help="go test command with optional extra arguments")
    p.add_argument('--fail-empty', action='store_true', help="crash if no tests in input")
    args = p.parse_args()
    logging.basicConfig(level=logging.DEBUG if args.debug else logging.INFO)

    if args.index < 0 or args.index >= args.nbins:
        raise ValueError('index out of range for nbins')

    with (sys.stdin if args.path == '-' else open(args.path)) as f_in:
        raw = f_in.read()
    sizes: SizeList = []
    cwd = os.getcwd()
    for item in iter_scan_once(raw):
        n_test_files = len(item.get('TestGoFiles', []) + item.get('XTestGoFiles', []))
        if n_test_files:
            sizes.append(('.' + item["Dir"].removeprefix(cwd), n_test_files))
    if not sizes:
        logger.warning('no tests to run, quitting')
        if args.fail_empty:
            raise ValueError('no tests in input and --fail-empty is set')
        return
    bin_size = int(sum(size for _, size in sizes) / args.nbins)
    logger.info('%d packages with tests, bin_size %d', len(sizes), bin_size)
    if PLACEMENT_RULES:
        sizes = apply_placement(sizes)
    splits = split_bins(sizes, bin_size)
    assert len(splits) == args.nbins
    logger.info('bin %d / %d has %d packages', args.index, args.nbins, len(splits[args.index]))
    if not args.dry_run:
        subprocess.run(f"{args.command} {' '.join(splits[args.index])}", shell=True, check=True)

if __name__ == '__main__':
    main()
