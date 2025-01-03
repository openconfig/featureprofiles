from ixnetwork_restpy.testplatform.testplatform import TestPlatform
from urllib.parse import urlparse
import datetime
import requests
import time
import sys

for target in sys.argv[1].split(','):
    u = urlparse(target)

    port = 443
    if u.port: port = u.port

    test_platform = TestPlatform(u.hostname, port)
    if not test_platform.Platform == "linux": continue

    test_platform.Authenticate(u.username, u.password)
    api_key = test_platform.ApiKey
    
    # graceful remove
    for session in test_platform.Sessions.find():
        created_on_str = session._properties['createdOn']
        created_on = datetime.datetime.fromisoformat(created_on_str)
        if created_on < datetime.datetime.now(created_on.tzinfo)-datetime.timedelta(hours=15):
            print(f"Removing stale session: {session.Id}")
            session.remove()
    
    time.sleep(5)

    # force delete
    for session in test_platform.Sessions.find():
        created_on_str = session._properties['createdOn']
        created_on = datetime.datetime.fromisoformat(created_on_str)
        if created_on < datetime.datetime.now(created_on.tzinfo)-datetime.timedelta(hours=15):
            print(f"Force deleting stale session: {session.Id}")
            api_url = f"{u.scheme}://{u.hostname}:{port}/ixnetworkweb/api/v1/sessions/{session.Id}"
            requests.delete(api_url, headers={"X-Api-Key": api_key}, verify=False)
