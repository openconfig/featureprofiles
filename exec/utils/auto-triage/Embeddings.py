from langchain_community.embeddings import HuggingFaceEmbeddings
import glob

class Embeddings:
    def __init__(self):
        self.name = "sentence-transformers/all-MiniLM-L6-v2"
        self.directory = "/auto/slapigo/firex/helpers/reporting/firex2mongo/models/all-MiniLM-L6-v2"

    def get_model(self):
        return HuggingFaceEmbeddings(model_name=self.directory)
    
    # not called anywhere atm
    def download(self):
        models = glob.glob(self.directory)
        if len(models) == 0:
            model = HuggingFaceEmbeddings(model_name=self.name)
            model.client.save_pretrained(self.directory)