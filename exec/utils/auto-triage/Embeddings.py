from langchain_community.embeddings import HuggingFaceEmbeddings
import glob

class Embeddings:
    def __init__(self):
        """Define the text embedding model and location of the local model"""
        self.name = "sentence-transformers/all-MiniLM-L6-v2"
        self.directory = "/auto/slapigo/firex/helpers/reporting/firex2mongo/models/all-MiniLM-L6-v2"

    def get_model(self):
        """Return the HuggingFace Text Embedding Model"""
        return HuggingFaceEmbeddings(model_name=self.directory)
    
    def download(self):
        """Download the model if not available (currently not used)"""
        models = glob.glob(self.directory)
        if len(models) == 0:
            model = HuggingFaceEmbeddings(model_name=self.name)
            model.client.save_pretrained(self.directory)