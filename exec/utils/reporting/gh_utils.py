from github import Github
import os

class FPGHRepo():
    def __init__(self):
        self._gh = Github(base_url="https://wwwin-github.cisco.com/api/v3",
                    login_or_token=os.environ['GH_TOKEN'])
        self._org = self._gh.get_organization("B4Test")
        self._repo = self._org.get_repo("featureprofiles")
        self._issues = list(self._repo.get_issues(state='open'))
        self._test_issue_map = {}

    def get_issue(self, test_name):
        if test_name in self._test_issue_map:
            return self._test_issue_map[test_name]

        for issue in self._issues:
            if issue.title.startswith(test_name):
                info = {
                    "number": issue.number,
                    "link": "https://wwwin-github.cisco.com/B4Test/featureprofiles/issues/" + str(issue.number),
                    "tags": [label.name for label in issue.get_labels()],
                    "bugs": []
                }

                for label in issue.get_labels():
                    if label.name.startswith("CSCw"):
                        info["bugs"].append({
                            'label': label.name,
                            'link': "https://cdetsng.cisco.com/summary/#/defect/" + label.name
                        })
                        
                self._test_issue_map[test_name] = info
                return info
        return None