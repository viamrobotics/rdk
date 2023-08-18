#!/usr/bin/env python3
"consume output of `go list -json` and run a partition of tests"

import argparse, json, sys, logging, subprocess
import os
from typing import List, Tuple

SizeList = List[Tuple[str, int]]
logger = logging.getLogger(__name__)

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
    splits = split_bins(sizes, bin_size)
    assert len(splits) == args.nbins
    logger.info('bin %d / %d has %d packages', args.index, args.nbins, len(splits[args.index]))
    if not args.dry_run:
        subprocess.run(f"{args.command} {splits[args.index].join(' ')}", shell=True, check=True)

if __name__ == '__main__':
    main()
