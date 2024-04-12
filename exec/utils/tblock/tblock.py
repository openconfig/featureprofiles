from pwd import getpwuid
from random import randint
import argparse
import yaml
import json
import time
import errno
import os

def _find_owner(filename):
    return getpwuid(os.stat(filename).st_uid).pw_name

def _lockfile(filename):
    try:
        os.close(os.open(filename, os.O_CREAT | os.O_EXCL | os.O_WRONLY));
    except OSError as e:
        if e.errno == errno.EEXIST:
            return False
        else:
            raise
    return True

def _lock_hw(hw):
    if type(hw) == str:
        return _lockfile(os.path.join(ldir, hw))
    
    locked = []
    for e in hw:
        if _lockfile(os.path.join(ldir, e)):
            locked.append(e)
        else:
            for lf in locked:
                os.remove(lf)
            return False
    return True

def _print_table(rows):
  max_col_lens = list(map(max, zip(*[(len(str(cell)) for cell in row) for row in rows])))
  print('┌' + '┬'.join('─' * (n + 2) for n in max_col_lens) + '┐')
  rows_separator = '├' + '┼'.join('─' * (n + 2) for n in max_col_lens) + '┤'
  row_fstring = ' │ '.join("{: <%s}" % n for n in max_col_lens)
  for i, row in enumerate(rows):
    print('│', row_fstring.format(*map(str, row)), '│')
    
    if i < len(rows) - 1:
      print(rows_separator)
  print('└' + '┴'.join('─' * (n + 2) for n in max_col_lens) + '┘')
        
def _show(available_only=False, json_output=False):
    data = [["Testbed", "Owner", "Available", "Reserved By"]]
    for t in testbeds:
        if t.get('sim', False):
            continue
        
        locked = False
        reserved_by = ''
        
        if type(t['hw']) == str:
            t['hw'] = [t['hw']]

        for e in t['hw']:
            lock_file = os.path.join(ldir, e)
            locked |= os.path.exists(lock_file)
            if locked:
                reserved_by =  _find_owner(lock_file) 
                break

        if not available_only or not locked:
            data.append([t['id'], t['owner'], not locked, reserved_by])
    if json_output:
        json_data = {
            'status': 'ok',
            'testbeds': []
        }
        for r in data[1:]:
            json_data['testbeds'].append({
                'id': r[0],
                'owner': r[1],
                'available': r[2],
                'reserved_by': r[3]
            })
        print(json.dumps(json_data))
    else:
        _print_table(data)

def _get_testbed(id):
    for tb in testbeds:
        if tb['id'] == id:
            return tb
    return None

def _trylock_helper(tb):
    if tb.get('sim', False):
        return True
    return _lock_hw(tb['hw'])

def _release_helper(tb):
    if tb.get('sim', False):
        return
    
    if type(tb['hw']) == str:
        tb['hw'] = [tb['hw']]

    for e in tb['hw']:
        lock_file = os.path.join(ldir, e)
        if os.path.exists(lock_file):
            os.remove(lock_file)

def _release_all(tbs):
    for tb in tbs:
        _release_helper(tb)

def _trylock(testbeds, wait=False, json_output=False):
    while True:
        locked = []
        for tb in testbeds:        
            if _trylock_helper(tb):
                locked.append(tb)
            else:
                _release_all(locked)
                break
        
        if len(locked) == len(testbeds):
            if json_output:
                print(json.dumps({'status': 'ok', 'testbeds': locked}))
            else:
                print(f"Success. Testbed(s) reserved.")
            exit(0)
        else: 
            if wait:
                time.sleep(randint(1,3))
            else:
                if json_output:
                    print(json.dumps({'status': 'fail'}))
                else:
                    print(f"Not all testbeds are available.")
                exit(1)
            
def _release(testbeds, json_output=False):
    _release_all(testbeds)
    if json_output: print(json.dumps({'status': 'ok'}))
    else: print(f"Testbed(s) released")

def _get_testbeds(ids, json_output=False):
    testbeds = []
    for id in ids.split(","):
        tb = _get_testbed(id)
        if not tb:
            if json_output: print(json.dumps({"status": "not found"}))
            else: print(f"Testbed '{id}' not found.")
            exit(1)
        testbeds.append(tb)
    return testbeds
    
def _is_valid_file(parser, arg):
    if not os.path.exists(arg):
        parser.error("File %s does not exist" % arg)
    else:
        return arg

if __name__ == "__main__":
    main_parser = argparse.ArgumentParser(description='B4 testbed reservation system')
    main_parser.add_argument('testbeds_file', type=lambda x: _is_valid_file(main_parser, x), help='path to testbeds.yaml file')
    main_parser.add_argument('locks_dir', type=str, help='path to locks directory')
    main_parser.add_argument('-j', '--json', action='store_true', default=False, help='json output')
    command_subparser = main_parser.add_subparsers(title='command', dest='command')
    command_subparser.required = True

    show_paser = command_subparser.add_parser('show', help='show all testbeds')
    show_paser.add_argument('-a', '--available', action='store_true', default=False, help='show available testbeds only')

    lock_parser = command_subparser.add_parser('lock', help='lock a testbed')
    lock_parser.add_argument('id', help='testbed id')
    lock_parser.add_argument('-w', '--wait',  default=False, action='store_true', help='wait until testbed is available')

    release_parser = command_subparser.add_parser('release', help='release a testbed')
    release_parser.add_argument('id', help='testbed id')

args = main_parser.parse_args()

ldir = args.locks_dir
if not os.path.exists(ldir):
    os.makedirs(ldir, mode=0o777, exist_ok=True)

testbeds = []
with open(args.testbeds_file, 'r') as fp:
    tf = yaml.safe_load(fp)
    tbs = tf['testbeds']
    for k in tbs:
        tbs[k]['id'] = k
        testbeds.append(tbs[k])

if args.command == 'show':
    _show(available_only=args.available, json_output=args.json)
elif args.command == 'lock':
    tbs = _get_testbeds(args.id, json_output=args.json)
    _trylock(tbs, args.wait, json_output=args.json)
elif args.command == 'release':
    tbs = _get_testbeds(args.id, json_output=args.json)
    _release(tbs, json_output=args.json)