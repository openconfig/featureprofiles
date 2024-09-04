from vectorstore import VectorStore
import argparse
from pymongo import MongoClient

includeGroups = ["b4-featureprofiles", "b4-internal"]

parser = argparse.ArgumentParser(description='Inject FireX Run Results in MongoDB')
parser.add_argument('run_id', help="FireX Run ID")
parser.add_argument('xunit_file', help="xUnit Result File")
parser.add_argument('--lineup', default='', help="Image Lineup")
parser.add_argument('--efr', default='', help="Image EFR")
parser.add_argument('--group', default='', help="Reporting Group")
args = parser.parse_args()

client = MongoClient("mongodb://xr-sf-npi-lnx.cisco.com:27017/")
database = client["auto-triage"]
groups = database["groups"]

watchlist = groups.find(filter = {}, projection = {
    "group": 1
})

watchlist_groups = [x["group"] for x in list(watchlist)]

if args.group in watchlist_groups:
    vs = VectorStore()
    documents = vs.create_documents(file = args.xunit_file, group = args.group, efr = args.efr, run_id = args.run_id, lineup = args.image_lineup)

    if len(documents) > 0:
        vs.insert_many(documents)
