from langchain_community.vectorstores import FAISS
from langchain_core.documents import Document
from Embeddings import Embeddings

embedding = Embeddings()

class Vectorstore:
    def __init__(self):
        # Retrieve text embedding model
        self._embedding_model = embedding.get_model()
        self._vector_store = None

    def create_index(self, documents = []):
        """Create FlatIndex from FAISS with required metadata"""
        if len(documents) == 0:
            return

        # Create individual documents, where each document is a single failed testcase
        vectors = [
            Document(
                page_content=x["logs"],
                metadata={
                    "name": x["name"],
                    "plan_id": x["plan_id"],
                    "timestamp": x["timestamp"],
                    "label": x["label"],
                },
            )
            for x in documents
        ]

        self._vector_store = FAISS.from_documents(vectors, self._embedding_model)
    
    def query(self, sentence):
        """Find up to 4 most similar and unique labeled documents"""
        if self._vector_store == None or sentence == None or sentence == "None":
            return []
    
        responses = self._vector_store.similarity_search_with_relevance_scores(
            sentence, k=4
        )

        labels = list()
        visited = set()

        for response in responses:
            data, score = response
            # Discard if the current response has already been seen or has a negative score
            if data.metadata["label"] in visited or score < 0:
                continue

            labels.append({"label": data.metadata["label"], "score": score})
            visited.add(data.metadata["label"])

        return labels
