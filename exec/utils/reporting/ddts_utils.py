from time import strftime, strptime
import requests

def get_ddts_info(name):
    bug_label = name
    bug_data = requests.get("http://wwwin-metrics.cisco.com/cgi-bin/ws/ws_ddts_query_new.cgi?expert=_id:%s&type=json&fields=New-on,Status,Est-fix-date,Identifier,Headline" % name).json()[0]
    bug_label += '/' + bug_data['Status']
    # if bug_data['Est-fix-date']:
    #     date = strptime(bug_data['Est-fix-date'], "%y%m%d")
    #     bug_label += ' (' + strftime("%m/%d/%Y", date) + ')'
    bug_data['custom_label'] = bug_label
    return bug_data
                