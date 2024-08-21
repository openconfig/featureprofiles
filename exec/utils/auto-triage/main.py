from vectorstore import VectorStore
import argparse

parser = argparse.ArgumentParser(description='Inject FireX Run Results in MongoDB')
parser.add_argument('run_id', help="FireX Run ID")
parser.add_argument('xunit_file', help="xUnit Result File")
parser.add_argument('--lineup', default='', help="Image Lineup")
parser.add_argument('--efr', default='', help="Image EFR")
parser.add_argument('--group', default='', help="Reporting Group")
args = parser.parse_args()

vs = VectorStore()
documents = vs.create_documents(file = args.xunit_file)

if len(documents) > 0:
    vs.insert_many(documents)
