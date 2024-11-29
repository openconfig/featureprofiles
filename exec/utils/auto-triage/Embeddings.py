from langchain_community.embeddings import HuggingFaceEmbeddings
import glob

class Embeddings:
    def get_model(self):
        directory = "./models/all-MiniLM-L6-v2"
        models = glob.glob(directory)

        if len(models) == 0:
            name = "sentence-transformers/all-MiniLM-L6-v2"
            model = HuggingFaceEmbeddings(model_name=name)
            model.client.save_pretrained(directory)
            return model
        
        return HuggingFaceEmbeddings(model_name=directory)